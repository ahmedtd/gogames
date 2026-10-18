package main

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/ahmedtd/gogames/games/renderino/internal/meshbvh"
	"github.com/ahmedtd/gogames/lib/vmath/fmat44"
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"

	"github.com/barnex/fmath"
)

// newRasterTestCamera makes a camera at center looking along eye, with its
// aperture fitted to an rx by ry image.
func newRasterTestCamera(center, eye fvec4.T, rx, ry int) *PinholeCamera {
	cam := &PinholeCamera{
		Center:          center,
		ApertureToWorld: fmat44.Identity(),
	}
	cam.SetEye(eye)
	cam.FitAperture(rx, ry)
	return cam
}

// cameraSampleRay returns the world-space camera ray through sample s of
// pixel (px, py), built the same way as renderFrame and RenderTile build it.
func cameraSampleRay(cam *PinholeCamera, px, py, s, samples, rx, ry int) Ray {
	ox, oy := supersampleOffset(s, samples)
	apertureRay := cam.ApertureRay(float32(px)+ox, float32(py)+oy, rx, ry)
	return Ray{
		Point: cam.Center,
		Slope: fmat44.MulMV(&cam.ApertureToWorld, apertureRay),
	}
}

// projectToClip carries world-space point p into clip space.  It works in
// float64, so that W stays accurate even for far-off vertices of huge
// triangles, where it's the small difference of large terms.
func projectToClip(proj RasterProjection, p fvec4.T) fvec4.T {
	var clip fvec4.T
	for r := range 4 {
		m := proj.WorldToClip[r*4 : r*4+4]
		clip[r] = float32(float64(m[0])*float64(p[0]) + float64(m[1])*float64(p[1]) + float64(m[2])*float64(p[2]) + float64(m[3]))
	}
	return clip
}

// TestRasterProjectionMatchesCameraRays checks that points along each camera
// ray project back onto that ray's sample position, with W equal to their
// depth along the camera's eye axis.
func TestRasterProjectionMatchesCameraRays(t *testing.T) {
	const rx, ry, samples = 160, 90, 4
	cam := newRasterTestCamera(fvec4.T{1, 2, 3}, fvec4.T{1, 0.5, -0.3}, rx, ry)
	proj := NewRasterProjection(cam, rx, ry)

	for _, px := range []int{0, 17, 80, 159} {
		for _, py := range []int{0, 44, 89} {
			for s := range samples {
				ox, oy := supersampleOffset(s, samples)
				ray := cameraSampleRay(cam, px, py, s, samples, rx, ry)
				for _, rayT := range []float32{0.5, 3, 40} {
					p := ray.Eval(rayT)
					clip := projectToClip(proj, p)
					gotX, gotY := clip[0]/clip[3], clip[1]/clip[3]
					wantX, wantY := float32(px)+ox, float32(py)+oy
					wantW := dot3(cam.Eye(), fvec4.SubVV(p, cam.Center))
					if fmath.Abs(gotX-wantX) > 1e-3 || fmath.Abs(gotY-wantY) > 1e-3 || fmath.Abs(clip[3]-wantW) > 1e-4*wantW {
						t.Errorf("pixel (%d, %d) sample %d, t=%v: got raster (%v, %v) W=%v; want (%v, %v) W=%v",
							px, py, s, rayT, gotX, gotY, clip[3], wantX, wantY, wantW)
					}
				}
			}
		}
	}
}

// TestRasterizeMatchesRaytrace rasterizes random triangles, including ones
// that cross the near plane, lie behind the camera, or extend far past the
// guard band, and checks that each covers the same samples as raytracing it
// would, at the same depth, facing, and barycentric coordinates.  Mismatches
// are allowed only for the few samples within snapping distance of an edge.
func TestRasterizeMatchesRaytrace(t *testing.T) {
	const rx, ry = 200, 150
	cam := newRasterTestCamera(fvec4.T{0, 0, 1}, fvec4.T{1, 0.2, -0.1}, rx, ry)
	proj := NewRasterProjection(cam, rx, ry)
	eye, left, up := cam.Eye(), cam.Left(), cam.Up()

	for _, samples := range []int{1, 4} {
		for _, cullBack := range []bool{false, true} {
			vb := NewGBuffer(40, 30, 104, 78, samples)
			rng := rand.New(rand.NewPCG(1, 2))

			var covered, mismatched int
			for i := range 300 {
				// Vertices are placed relative to the camera, mostly in
				// front of it, and every tenth triangle is huge.
				scale := float32(1)
				if i%10 == 0 {
					scale = 1e4
				}
				var vs [3]fvec4.T
				var clip [3]fvec4.T
				for j := range vs {
					d := fvec4.AddVV(fvec4.AddVV(
						fvec4.MulVS(eye, -2+12*rng.Float32()),
						fvec4.MulVS(left, scale*(-4+8*rng.Float32()))),
						fvec4.MulVS(up, scale*(-3+6*rng.Float32())))
					vs[j] = fvec4.AddVV(cam.Center, d)
					vs[j][3] = 0
					clip[j] = projectToClip(proj, vs[j])
				}
				tri := MeshTri{Triangle: meshbvh.NewTriangle(vs[0], vs[1], vs[2])}
				frontFacing := dot3(fvec4.CProd3(tri.E1, tri.E2), fvec4.SubVV(vs[0], cam.Center)) < 0

				// Each vertex's normal is a different basis vector, so the
				// interpolated normal is the barycentric coordinates.  It's
				// negated on back faces, to face the camera.
				attrs := RasterAttrs{
					Normals:    [3]fvec4.T{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}},
					GeomNormal: fvec4.T{1, 1, 1, 0},
				}

				vb.Clear()
				vb.RasterizeTri(clip, &attrs, cullBack, VisHit{Tri: int32(i)})

				for py := vb.SrcY; py < vb.LimY; py++ {
					for px := vb.SrcX; px < vb.LimX; px++ {
						for s := range samples {
							idx := ((py-vb.SrcY)*(vb.LimX-vb.SrcX)+(px-vb.SrcX))*samples + s
							got := vb.Hits[idx] != visHitNone

							ray := cameraSampleRay(cam, px, py, s, samples, rx, ry)
							_, _, _, ok := tri.RayIntersect(ray.Point, ray.Slope, 0, float32(math.Inf(1)))
							wantW, wantB1, wantB2 := planeHit(vs, ray, eye)
							want := ok && wantW >= rasterNear && (frontFacing || !cullBack)

							if got || want {
								covered++
							}
							if got != want {
								mismatched++
								continue
							}
							if !got {
								continue
							}
							if fmath.Abs(vb.InvDepth[idx]*wantW-1) > 1e-4 {
								t.Errorf("samples=%d cullBack=%v tri %d: pixel (%d, %d) sample %d has inverse depth %v; want %v",
									samples, cullBack, i, px, py, s, vb.InvDepth[idx], 1/wantW)
							}
							if vb.Hits[idx].Back != !frontFacing {
								t.Errorf("samples=%d cullBack=%v tri %d: pixel (%d, %d) sample %d has Back=%v; want %v",
									samples, cullBack, i, px, py, s, vb.Hits[idx].Back, !frontFacing)
							}
							wantN := [3]float32{1 - wantB1 - wantB2, wantB1, wantB2}
							if !frontFacing {
								wantN = [3]float32{-wantN[0], -wantN[1], -wantN[2]}
							}
							gotN := vb.Normals[idx]
							if fmath.Abs(gotN[0]-wantN[0]) > 1e-4 || fmath.Abs(gotN[1]-wantN[1]) > 1e-4 || fmath.Abs(gotN[2]-wantN[2]) > 1e-4 {
								t.Errorf("samples=%d cullBack=%v tri %d: pixel (%d, %d) sample %d has normal %v; want %v",
									samples, cullBack, i, px, py, s, gotN, wantN)
							}
						}
					}
				}
			}

			t.Logf("samples=%d cullBack=%v: %d of %d covered samples mismatched", samples, cullBack, mismatched, covered)
			if covered == 0 || float64(mismatched) > 1e-3*float64(covered) {
				t.Errorf("samples=%d cullBack=%v: %d of %d covered samples mismatched", samples, cullBack, mismatched, covered)
			}
		}
	}
}

// planeHit returns the eye-axis depth at which ray meets the plane of triangle
// vs, and the barycentric coordinates there with respect to vertices 1 and 2.
// It works in float64, so that it's an accurate reference even for huge
// triangles.
func planeHit(vs [3]fvec4.T, ray Ray, eye fvec4.T) (w, b1, b2 float32) {
	var v [3][3]float64
	for i := range v {
		for k := range 3 {
			v[i][k] = float64(vs[i][k])
		}
	}
	a := [3]float64{v[1][0] - v[0][0], v[1][1] - v[0][1], v[1][2] - v[0][2]}
	b := [3]float64{v[2][0] - v[0][0], v[2][1] - v[0][1], v[2][2] - v[0][2]}
	n := [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
	var num, den, eyeSlope float64
	for k := range 3 {
		num += n[k] * (v[0][k] - float64(ray.Point[k]))
		den += n[k] * float64(ray.Slope[k])
		eyeSlope += float64(eye[k]) * float64(ray.Slope[k])
	}
	t := num / den

	// p - v0 = b1*a + b2*b, so (p - v0) x b = b1*n and a x (p - v0) = b2*n.
	var pv [3]float64
	for k := range 3 {
		pv[k] = float64(ray.Point[k]) + t*float64(ray.Slope[k]) - v[0][k]
	}
	nn := n[0]*n[0] + n[1]*n[1] + n[2]*n[2]
	pvb := cross3(pv, b)
	apv := cross3(a, pv)
	b1 = float32((pvb[0]*n[0] + pvb[1]*n[1] + pvb[2]*n[2]) / nn)
	b2 = float32((apv[0]*n[0] + apv[1]*n[1] + apv[2]*n[2]) / nn)
	return float32(t * eyeSlope), b1, b2
}

// coverageCounts rasterizes each triangle on its own, returning how many
// triangles covered each sample of vb.
func coverageCounts(vb *GBuffer, tris [][3]fvec4.T) []int {
	counts := make([]int, len(vb.Hits))
	vb.Clear()
	for i, clip := range tris {
		vb.RasterizeTri(clip, &RasterAttrs{}, false, VisHit{Tri: int32(i)})
		for j, h := range vb.Hits {
			if h != visHitNone {
				counts[j]++
				vb.Hits[j] = visHitNone
				vb.InvDepth[j] = 0
			}
		}
	}
	return counts
}

// TestRasterizeWatertightGrid rasterizes a jittered grid of triangles that
// covers the whole tile, with vertices and edges lying exactly on sample
// positions, and checks that every sample is covered exactly once.
func TestRasterizeWatertightGrid(t *testing.T) {
	for _, samples := range []int{1, 4} {
		vb := NewGBuffer(8, 8, 40, 32, samples)
		rng := rand.New(rand.NewPCG(3, 4))

		// Grid vertices are 4 pixels apart, jittered by multiples of a
		// quarter pixel, so that many fall exactly on sample positions
		// (which are multiples of 1/8 pixel) and many grid lines are exactly
		// horizontal or vertical.  W is a power of two, so the perspective
		// divide is exact.
		const n = 14
		var grid [n][n]fvec4.T
		for i := range n {
			for j := range n {
				x := float32(i*4) + 0.5 + float32(rng.IntN(5)-2)/4
				y := float32(j*4) + 0.5 + float32(rng.IntN(5)-2)/4
				w := []float32{0.5, 1, 2, 4}[rng.IntN(4)]
				grid[i][j] = fvec4.T{x * w, y * w, 0, w}
			}
		}
		var tris [][3]fvec4.T
		for i := range n - 1 {
			for j := range n - 1 {
				a, b, c, d := grid[i][j], grid[i+1][j], grid[i+1][j+1], grid[i][j+1]
				if rng.IntN(2) == 0 {
					tris = append(tris, [3]fvec4.T{a, b, c}, [3]fvec4.T{a, c, d})
				} else {
					tris = append(tris, [3]fvec4.T{a, b, d}, [3]fvec4.T{b, c, d})
				}
			}
		}

		for i, c := range coverageCounts(vb, tris) {
			if c != 1 {
				t.Errorf("samples=%d: sample %d covered %d times; want 1", samples, i, c)
			}
		}
	}
}

// TestRasterizeWatertightFloor rasterizes a jittered floor grid that passes
// under and behind the camera, so that many of its triangles are clipped by
// the near plane and the guard band, and checks that no sample is covered
// twice and every sample whose camera ray hits the floor is covered once.
func TestRasterizeWatertightFloor(t *testing.T) {
	const rx, ry, samples = 120, 90, 2
	cam := newRasterTestCamera(fvec4.T{0, 0, 1}, fvec4.T{1, 0.3, -0.4}, rx, ry)
	proj := NewRasterProjection(cam, rx, ry)
	rng := rand.New(rand.NewPCG(5, 6))

	// A 40x40 grid of floor points at z = 0, spanning [-40, 40] in x and y,
	// so that it extends well behind the camera.
	const n = 41
	const extent = 40
	var grid [n][n]fvec4.T
	for i := range n {
		for j := range n {
			jx := 0.8 * (rng.Float32() - 0.5)
			jy := 0.8 * (rng.Float32() - 0.5)
			if i == 0 || i == n-1 {
				jx = 0
			}
			if j == 0 || j == n-1 {
				jy = 0
			}
			p := fvec4.T{float32(2*i-(n-1)) + 2*jx, float32(2*j-(n-1)) + 2*jy, 0, 0}
			grid[i][j] = projectToClip(proj, p)
		}
	}
	var tris [][3]fvec4.T
	for i := range n - 1 {
		for j := range n - 1 {
			a, b, c, d := grid[i][j], grid[i+1][j], grid[i+1][j+1], grid[i][j+1]
			tris = append(tris, [3]fvec4.T{a, b, c}, [3]fvec4.T{a, c, d})
		}
	}

	// Check one tile above the horizon, one across it, and one at the
	// bottom of the image, nearest the camera.
	for _, tile := range [][4]int{{0, 0, 32, 24}, {44, 30, 76, 54}, {80, 66, 120, 90}} {
		vb := NewGBuffer(tile[0], tile[1], tile[2], tile[3], samples)
		counts := coverageCounts(vb, tris)
		var hitFloor int
		for py := vb.SrcY; py < vb.LimY; py++ {
			for px := vb.SrcX; px < vb.LimX; px++ {
				for s := range samples {
					idx := ((py-vb.SrcY)*(vb.LimX-vb.SrcX)+(px-vb.SrcX))*samples + s
					c := counts[idx]

					// Where does this sample's ray meet the floor?  Rays
					// meeting it well inside the grid's (unjittered)
					// border must be covered.
					ray := cameraSampleRay(cam, px, py, s, samples, rx, ry)
					insideGrid := false
					if ray.Slope[2] < 0 {
						p := ray.Eval(-ray.Point[2] / ray.Slope[2])
						insideGrid = fmath.Abs(p[0]) < extent-1 && fmath.Abs(p[1]) < extent-1
					}
					if insideGrid {
						hitFloor++
					}

					if c > 1 || (insideGrid && c != 1) {
						t.Errorf("tile %v: pixel (%d, %d) sample %d covered %d times (ray inside grid: %v)", tile, px, py, s, c, insideGrid)
					}
				}
			}
		}
		t.Logf("tile %v: %d samples hit the floor", tile, hitFloor)
	}
}

// TestRasterizeDepthOrder checks that the nearer of two overlapping triangles
// wins, whichever is rasterized first.
func TestRasterizeDepthOrder(t *testing.T) {
	near := [3]fvec4.T{{0, 0, 0, 1}, {20, 0, 0, 1}, {0, 20, 0, 1}}
	far := [3]fvec4.T{{0, 0, 0, 2}, {40, 0, 0, 2}, {0, 40, 0, 2}}
	for _, order := range [][2]int{{0, 1}, {1, 0}} {
		vb := NewGBuffer(0, 0, 16, 16, 1)
		tris := [2][3]fvec4.T{near, far}
		for _, i := range order {
			vb.RasterizeTri(tris[i], &RasterAttrs{}, false, VisHit{Tri: int32(i)})
		}
		// Pixel (2, 2) is covered by both, at inverse depths 1 and 0.5.
		idx := 2*16 + 2
		if vb.Hits[idx].Tri != 0 || vb.InvDepth[idx] != 1 {
			t.Errorf("order %v: pixel (2, 2) has hit %v at inverse depth %v; want triangle 0 at 1", order, vb.Hits[idx], vb.InvDepth[idx])
		}
	}
}
