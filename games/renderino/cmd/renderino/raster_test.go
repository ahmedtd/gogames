package main

import (
	"math/rand/v2"
	"testing"

	"github.com/ahmedtd/gogames/lib/graphics/pinholecamera"
	"github.com/ahmedtd/gogames/lib/vmath/fmat33"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"

	"github.com/barnex/fmath"
)

// newRasterTestCamera makes a camera at center looking along eye, with its
// aperture fitted to an rx by ry image.
func newRasterTestCamera(center, eye fvec3.T, rx, ry int) *pinholecamera.PinholeCamera {
	cam := &pinholecamera.PinholeCamera{
		Center:          center,
		ApertureToWorld: fmat33.Identity(),
	}
	cam.SetEye(eye)
	cam.FitAperture(rx, ry)
	return cam
}

// cameraSampleRay returns the world-space camera ray through sample s of
// pixel (px, py), built the same way as renderFrame and RenderTile build it.
func cameraSampleRay(cam *pinholecamera.PinholeCamera, px, py, s, samples, rx, ry int) Ray {
	ox, oy := supersampleOffset(s, samples)
	apertureRay := cam.ApertureRay(float32(px)+ox, float32(py)+oy, rx, ry)
	slope := fmat33.MulMV(cam.ApertureToWorld, apertureRay)
	return Ray{
		Point: fvec4.T{cam.Center[0], cam.Center[1], cam.Center[2], 0},
		Slope: fvec4.T{slope[0], slope[1], slope[2]},
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
	cam := newRasterTestCamera(fvec3.T{0, 0, 1}, fvec3.T{1, 0.3, -0.4}, rx, ry)
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
