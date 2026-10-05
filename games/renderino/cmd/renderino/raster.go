package main

import (
	"math"

	"github.com/ahmedtd/gogames/lib/vmath/fmat44"
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// Rasterization finds the nearest triangle under each camera sample, as a
// faster replacement for tracing primary rays.  It's adapted from
// softrenderer's rasterizer (lib/graphics/scene3), with these changes:
//
//   - The projection is built from PinholeCamera, so that samples land exactly
//     where the camera rays in RenderTile.CameraRays point.
//   - Triangles are clipped against a near plane (and a guard band far outside
//     the image) in homogeneous clip space before the perspective divide.
//   - Vertices are snapped to a fixed-point subpixel grid and edge functions
//     are evaluated incrementally with exact integer math, with a top-left
//     fill rule.  So triangles sharing an edge cover each sample along it
//     exactly once: there are no cracks or double hits.
//   - Only the triangle's identity and depth are recorded, in a VisBuffer.
//     Shading reconstructs the contact by intersecting the camera ray with
//     that one triangle, so no attributes are interpolated here.
//
// Clip space here is (X, Y, unused, W), where W is the depth along the
// camera's eye axis, and X/W and Y/W are raster positions in pixels.

const (
	// Raster positions are snapped to 1/rasterSubpixels of a pixel.
	rasterSubpixelBits = 8
	rasterSubpixels    = 1 << rasterSubpixelBits

	// rasterNear is the smallest W (eye-axis depth) that is rasterized.
	// Anything nearer to the camera is clipped away.
	rasterNear = 0.01

	// rasterGuardBand bounds the raster positions of snapped vertices, in
	// pixels.  Triangles reaching past it are clipped to it.  It keeps the
	// integer edge functions well clear of overflow: snapped coordinates fit
	// in 29 bits, so their products fit in 59.
	rasterGuardBand = 1 << 20
)

// RasterProjection carries world-space points into clip space for an rx by ry
// image seen through a PinholeCamera.
type RasterProjection struct {
	WorldToClip fmat44.T
}

// NewRasterProjection builds the projection matching cam's camera rays (see
// PinholeCamera.ApertureRay) for an rx by ry image.  cam's aperture must
// already be fitted to the image.
func NewRasterProjection(cam *PinholeCamera, rx, ry int) RasterProjection {
	// A point at aperture-space offset d = (eye, left, up) . (p - Center) from
	// the camera lies on the camera ray through raster x where
	//
	//	(1 - 2x/rx) * Aperture[1] / Aperture[0] = d.left / d.eye
	//
	// so x * d.eye = rx/2 * (d.eye - Aperture[0]/Aperture[1] * d.left), and
	// likewise for y with up and Aperture[2].
	eye, left, up := cam.Eye(), cam.Left(), cam.Up()
	kx := cam.Aperture[0] / cam.Aperture[1]
	ky := cam.Aperture[0] / cam.Aperture[2]
	rowX := fvec4.MulVS(fvec4.SubVV(eye, fvec4.MulVS(left, kx)), float32(rx)/2)
	rowY := fvec4.MulVS(fvec4.SubVV(eye, fvec4.MulVS(up, ky)), float32(ry)/2)
	c := cam.Center
	return RasterProjection{
		WorldToClip: fmat44.T{
			rowX[0], rowX[1], rowX[2], -dot3(rowX, c),
			rowY[0], rowY[1], rowY[2], -dot3(rowY, c),
			0, 0, 0, 0,
			eye[0], eye[1], eye[2], -dot3(eye, c),
		},
	}
}

// rasterTilePlanes returns the planes bounding the part of space that lands in
// raster pixels [srcX, limX) x [srcY, limY), nearer than rasterNear excluded,
// for use with bvh.Tree.FrustumTest.  toClip carries points into clip space;
// the planes are in whatever space it carries points from, so passing a
// model-to-clip transform gives model-space planes.  The near plane comes
// first, so that FrustumTest visits nearer bounds first.
func rasterTilePlanes(toClip *fmat44.T, srcX, srcY, limX, limY int) [5]fvec4.T {
	// For W > 0, x = X/W >= srcX exactly when X - srcX*W >= 0, and so on;
	// each of these is a linear combination of rows of toClip.
	rowX := fvec4.T{toClip[0], toClip[1], toClip[2], toClip[3]}
	rowY := fvec4.T{toClip[4], toClip[5], toClip[6], toClip[7]}
	rowW := fvec4.T{toClip[12], toClip[13], toClip[14], toClip[15]}
	combine := func(a fvec4.T, ka float32, b fvec4.T, kb float32) fvec4.T {
		return fvec4.T{ka*a[0] + kb*b[0], ka*a[1] + kb*b[1], ka*a[2] + kb*b[2], ka*a[3] + kb*b[3]}
	}
	near := rowW
	near[3] -= rasterNear
	return [5]fvec4.T{
		near,
		combine(rowX, 1, rowW, -float32(srcX)),
		combine(rowX, -1, rowW, float32(limX)),
		combine(rowY, 1, rowW, -float32(srcY)),
		combine(rowY, -1, rowW, float32(limY)),
	}
}

// VisHit identifies a triangle: Tri indexes the Tris of the MeshModel of
// MeshInstances[Inst].
type VisHit struct {
	Inst, Tri int32
}

// RasterStats counts rasterization work.  Like RayStats, it's plain counters,
// so each render thread should count into its own.
type RasterStats struct {
	Tris          int64 // Triangles passed to RasterizeTri.
	TrisSetUp     int64 // Triangles (after clipping) that reached the tile and were set up.
	SamplesTested int64 // Samples in set-up triangles' bounding boxes.
	SamplesHit    int64 // Samples covered and passing the depth test.
}

// Add adds o's counts into s.
func (s *RasterStats) Add(o *RasterStats) {
	s.Tris += o.Tris
	s.TrisSetUp += o.TrisSetUp
	s.SamplesTested += o.SamplesTested
	s.SamplesHit += o.SamplesHit
}

// visHitNone marks a sample that no triangle covers.
var visHitNone = VisHit{-1, -1}

// VisBuffer is a tile's visibility buffer: for each camera sample in the tile,
// the nearest triangle rasterized over it so far.  Samples are laid out like
// RenderTile.CameraRays: row-major by pixel, with each pixel's samples
// contiguous.
type VisBuffer struct {
	SrcX, SrcY int
	LimX, LimY int
	Samples    int

	// Work done rasterizing into the buffer since it was last cleared.
	Stats RasterStats

	// Per sample, the inverse depth (1/W) of the nearest triangle, or 0 if
	// none covers it.  Kept apart from Hits so the depth test reads densely.
	InvDepth []float32
	Hits     []VisHit

	// Position of each sample within its pixel, in subpixels from the
	// pixel's top-left corner.
	sampleX, sampleY []int64

	// Scratch space for each edge's per-sample edge function offsets, and
	// the per-sample inverse depth offsets.
	edgeOffsets  [3][]int64
	depthOffsets []float32
}

// NewVisBuffer makes an empty VisBuffer covering pixels [srcX, limX) x [srcY,
// limY), with samples per pixel placed by supersampleOffset.
func NewVisBuffer(srcX, srcY, limX, limY, samples int) *VisBuffer {
	n := (limX - srcX) * (limY - srcY) * samples
	vb := &VisBuffer{
		SrcX:     srcX,
		SrcY:     srcY,
		LimX:     limX,
		LimY:     limY,
		Samples:  samples,
		InvDepth: make([]float32, n),
		Hits:     make([]VisHit, n),
		sampleX:  make([]int64, samples),
		sampleY:  make([]int64, samples),

		depthOffsets: make([]float32, samples),
	}
	for s := range samples {
		ox, oy := supersampleOffset(s, samples)
		vb.sampleX[s] = int64(math.Round(float64(ox) * rasterSubpixels))
		vb.sampleY[s] = int64(math.Round(float64(oy) * rasterSubpixels))
	}
	for e := range vb.edgeOffsets {
		vb.edgeOffsets[e] = make([]int64, samples)
	}
	vb.Clear()
	return vb
}

// Clear empties every sample, and resets Stats.
func (vb *VisBuffer) Clear() {
	vb.Stats = RasterStats{}
	clear(vb.InvDepth)
	for i := range vb.Hits {
		vb.Hits[i] = visHitNone
	}
}

// RasterizeTri depth-tests a clip-space triangle against every sample it
// covers, recording hit at the samples where it's nearest.  If cullBack is
// set, triangles facing away from the camera (by the winding of their
// vertices, as in MeshTri) are skipped.
func (vb *VisBuffer) RasterizeTri(clip [3]fvec4.T, cullBack bool, hit VisHit) {
	vb.Stats.Tris++

	// Trivially reject triangles entirely outside one clip plane, and pass
	// triangles entirely inside all of them straight through.
	var outside [3]uint8
	for i, v := range clip {
		for p := range numClipPlanes {
			if clipDist(v, p) < 0 {
				outside[i] |= 1 << p
			}
		}
	}
	if outside[0]&outside[1]&outside[2] != 0 {
		return
	}
	if outside[0]|outside[1]|outside[2] == 0 {
		vb.rasterizeClipped(clip[0], clip[1], clip[2], cullBack, hit)
		return
	}

	// Clip to each plane in turn, then fan the remaining convex polygon
	// back into triangles.  Clipping preserves winding.
	var bufA, bufB [3 + numClipPlanes]fvec4.T
	poly := append(bufA[:0], clip[:]...)
	scratch := bufB[:0]
	for p := range numClipPlanes {
		if (outside[0]|outside[1]|outside[2])&(1<<p) == 0 {
			continue
		}
		scratch = clipPolygon(poly, p, scratch[:0])
		poly, scratch = scratch, poly
		if len(poly) < 3 {
			return
		}
	}
	for i := 1; i+1 < len(poly); i++ {
		vb.rasterizeClipped(poly[0], poly[i], poly[i+1], cullBack, hit)
	}
}

const numClipPlanes = 5

// clipDist returns how far inside clip plane p clip-space point v is, with
// negative values outside.  Plane 0 is the near plane, and 1-4 are the guard
// band's sides.
func clipDist(v fvec4.T, p int) float32 {
	switch p {
	case 0:
		return v[3] - rasterNear
	case 1:
		return rasterGuardBand*v[3] - v[0]
	case 2:
		return rasterGuardBand*v[3] + v[0]
	case 3:
		return rasterGuardBand*v[3] - v[1]
	default:
		return rasterGuardBand*v[3] + v[1]
	}
}

// clipPolygon clips convex polygon in against clip plane p
// (Sutherland-Hodgman), appending the result to out.
func clipPolygon(in []fvec4.T, p int, out []fvec4.T) []fvec4.T {
	for i := range in {
		a, b := in[i], in[(i+1)%len(in)]
		da, db := clipDist(a, p), clipDist(b, p)
		if da >= 0 {
			out = append(out, a)
		}
		if (da >= 0) == (db >= 0) {
			continue
		}

		// Always interpolate from the inside vertex, so that the two
		// triangles sharing this edge (which walk it in opposite
		// directions) compute bit-identical intersections, and stay
		// watertight.
		vIn, vOut, dIn, dOut := a, b, da, db
		if da < 0 {
			vIn, vOut, dIn, dOut = b, a, db, da
		}
		t := dIn / (dIn - dOut)
		out = append(out, fvec4.T{
			vIn[0] + t*(vOut[0]-vIn[0]),
			vIn[1] + t*(vOut[1]-vIn[1]),
			0,
			vIn[3] + t*(vOut[3]-vIn[3]),
		})
	}
	return out
}

// rasterizeClipped rasterizes a triangle whose vertices are all inside the
// clip planes.
func (vb *VisBuffer) rasterizeClipped(c0, c1, c2 fvec4.T, cullBack bool, hit VisHit) {
	// Perspective divide, and snap to the subpixel grid for coverage.
	// Inverse depth is linear in raster space, so it's interpolated as a
	// plane, fitted to the unsnapped vertices: near the near plane, inverse
	// depth changes so steeply that snapping would skew it noticeably.
	var x, y [3]int64
	var fx, fy, fz [3]float64
	for i, c := range [3]fvec4.T{c0, c1, c2} {
		invW := 1 / float64(c[3])
		fx[i] = float64(c[0]) * invW
		fy[i] = float64(c[1]) * invW
		fz[i] = invW
		x[i] = int64(math.Round(fx[i] * rasterSubpixels))
		y[i] = int64(math.Round(fy[i] * rasterSubpixels))
	}

	// Raster y points down, so triangles facing the camera wind clockwise
	// on screen, giving a negative area here.  Swap two vertices of those so
	// that every triangle's edge functions are positive inside.
	area := (x[1]-x[0])*(y[2]-y[0]) - (y[1]-y[0])*(x[2]-x[0])
	switch {
	case area == 0:
		return
	case area > 0:
		if cullBack {
			return
		}
	default:
		x[1], x[2] = x[2], x[1]
		y[1], y[2] = y[2], y[1]
		area = -area
	}

	// Pixel bounds of the triangle, clamped to the tile.  Pixel p's samples
	// lie in subpixels [p*rasterSubpixels, (p+1)*rasterSubpixels).
	px0 := max(int(min(x[0], x[1], x[2])>>rasterSubpixelBits), vb.SrcX)
	px1 := min(int(max(x[0], x[1], x[2])>>rasterSubpixelBits), vb.LimX-1)
	py0 := max(int(min(y[0], y[1], y[2])>>rasterSubpixelBits), vb.SrcY)
	py1 := min(int(max(y[0], y[1], y[2])>>rasterSubpixelBits), vb.LimY-1)
	if px0 > px1 || py0 > py1 {
		return
	}

	// Edge e runs between the two vertices other than e, and its edge
	// function is proportional to vertex e's barycentric coordinate:
	//
	//	E(p) = (xb-xa)*(py-ya) - (yb-ya)*(px-xa)
	//
	// evaluated here at the subpixel origin of pixel (px0, py0), and stepped
	// per pixel and per sample from there.
	originX := int64(px0) << rasterSubpixelBits
	originY := int64(py0) << rasterSubpixelBits
	var rowE, stepX, stepY [3]int64
	for e := range 3 {
		a, b := (e+1)%3, (e+2)%3
		dx, dy := x[b]-x[a], y[b]-y[a]
		rowE[e] = dx*(originY-y[a]) - dy*(originX-x[a])

		// Top-left fill rule: a sample exactly on an edge belongs to the
		// triangle for which that edge is a top or left edge.  The two
		// triangles sharing an edge walk it in opposite directions, so
		// exactly one of them counts it.
		if !(dy < 0 || (dy == 0 && dx > 0)) {
			rowE[e]--
		}

		stepX[e] = -dy << rasterSubpixelBits
		stepY[e] = dx << rasterSubpixelBits
		for s := range vb.Samples {
			vb.edgeOffsets[e][s] = dx*vb.sampleY[s] - dy*vb.sampleX[s]
		}
	}

	// Fit the inverse depth plane z = dzdx*x + dzdy*y + z00, with x and y
	// in pixels relative to the origin of pixel (px0, py0).  Its normal is
	// the cross product of two edges in (x, y, z).
	ax, ay, az := fx[1]-fx[0], fy[1]-fy[0], fz[1]-fz[0]
	bx, by, bz := fx[2]-fx[0], fy[2]-fy[0], fz[2]-fz[0]
	nx, ny, nz := ay*bz-az*by, az*bx-ax*bz, ax*by-ay*bx
	if nz == 0 {
		return
	}
	dzdx64, dzdy64 := -nx/nz, -ny/nz
	dzdx, dzdy := float32(dzdx64), float32(dzdy64)
	z00 := float32(fz[0] + dzdx64*(float64(px0)-fx[0]) + dzdy64*(float64(py0)-fy[0]))

	n := vb.Samples
	vb.Stats.TrisSetUp++
	vb.Stats.SamplesTested += int64((px1-px0+1)*(py1-py0+1)) * int64(n)
	var samplesHit int64

	for s := range n {
		vb.depthOffsets[s] = dzdx*float32(vb.sampleX[s])/rasterSubpixels + dzdy*float32(vb.sampleY[s])/rasterSubpixels
	}

	off0 := vb.edgeOffsets[0][:n]
	off1 := vb.edgeOffsets[1][:n]
	off2 := vb.edgeOffsets[2][:n]
	offZ := vb.depthOffsets[:n]
	tileW := vb.LimX - vb.SrcX
	for py := py0; py <= py1; py++ {
		e0, e1, e2 := rowE[0], rowE[1], rowE[2]
		rowZ := z00 + dzdy*float32(py-py0)
		i := ((py-vb.SrcY)*tileW + (px0 - vb.SrcX)) * n
		for px := px0; px <= px1; px++ {
			pixelZ := rowZ + dzdx*float32(px-px0)
			depths := vb.InvDepth[i : i+n]
			hits := vb.Hits[i : i+n]
			for s := range n {
				w0 := e0 + off0[s]
				w1 := e1 + off1[s]
				w2 := e2 + off2[s]
				if w0|w1|w2 < 0 {
					continue
				}
				d := pixelZ + offZ[s]
				if d <= depths[s] {
					continue
				}
				depths[s] = d
				hits[s] = hit
				samplesHit++
			}
			i += n
			e0 += stepX[0]
			e1 += stepX[1]
			e2 += stepX[2]
		}
		rowE[0] += stepY[0]
		rowE[1] += stepY[1]
		rowE[2] += stepY[2]
	}
	vb.Stats.SamplesHit += samplesHit
}
