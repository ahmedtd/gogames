package main

import (
	"math"

	"github.com/ahmedtd/gogames/lib/graphics/renderable3"
	"github.com/ahmedtd/gogames/lib/vmath/fmat44"
	"github.com/ahmedtd/gogames/lib/vmath/fvec2"
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
//   - Vertex attributes are interpolated perspective-correctly, and the
//     nearest surface's depth, normal, and albedo are recorded in a GBuffer
//     for deferred shading.
//
// Clip space here is (X, Y, unused, W), where W is the depth along the
// camera's eye axis, and X/W and Y/W are raster positions in pixels.
//
// Perspective-correct interpolation uses 2D homogeneous coordinates (Olano
// and Greer, "Triangle Scan Conversion using 2D Homogeneous Coordinates").
// With vi = (Xi, Yi, Wi) the clip-space vertices, the functions
//
//	Ei(x, y) = (vj x vk) . (x, y, 1)	for (i, j, k) cyclic
//
// are affine in raster space, and at raster position (x, y):
//
//   - E0 + E1 + E2 = D/W, where D = v0 . (v1 x v2), so inverse depth is
//     (E0 + E1 + E2)/D;
//   - the triangle's barycentric coordinates (in world space, not raster
//     space) are Ei / (E0 + E1 + E2).
//
// So each fragment evaluates three affine functions and does one divide.
// These are in terms of the original triangle, so they're unaffected by
// clipping, and the sign of D gives the triangle's facing even before
// clipping.  This is the same result as section 11.3.1 of Fundamentals of
// Computer Graphics, which interpolates attributes and 1/W linearly in raster
// space and divides.

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
// MeshInstances[Inst].  Back is set if the camera sees the triangle's back.
type VisHit struct {
	Inst, Tri int32
	Back      bool
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
var visHitNone = VisHit{-1, -1, false}

// RasterAttrs are a triangle's vertex attributes, interpolated across it by
// RasterizeTri.
type RasterAttrs struct {
	// World-space shading normal at each vertex, not necessarily
	// normalized.
	Normals [3]fvec4.T

	// World-space geometric normal, on the side the vertices wind
	// counterclockwise around (as in MeshTri), not necessarily normalized.
	GeomNormal fvec4.T

	// Texture coordinates of each vertex in Diffuse; see sampleDiffuse.
	UVs     [3]fvec2.T
	Diffuse *renderable3.UncompressedImage
}

// GBuffer is a tile's geometry buffer: for each camera sample in the tile, the
// nearest triangle rasterized over it so far, and its surface there.  Samples
// are laid out like RenderTile.CameraRays: row-major by pixel, with each
// pixel's samples contiguous.
type GBuffer struct {
	SrcX, SrcY int
	LimX, LimY int
	Samples    int

	// Work done rasterizing into the buffer since it was last cleared.
	Stats RasterStats

	// Per sample, the inverse depth (1/W) of the nearest triangle, or 0 if
	// none covers it.  Kept apart from the rest so the depth test reads
	// densely.
	InvDepth []float32
	Hits     []VisHit

	// Per sample covered by a triangle, its interpolated world-space
	// shading normal (not normalized), facing the camera, and its albedo.
	Normals [][3]float32
	Albedos []RGBPower

	// Position of each sample within its pixel, in subpixels from the
	// pixel's top-left corner.
	sampleX, sampleY []int64

	// Scratch space for each edge's per-sample edge function offsets, and
	// each interpolation plane's per-sample offsets.
	edgeOffsets  [3][]int64
	planeOffsets [3][]float32
}

// NewGBuffer makes an empty GBuffer covering pixels [srcX, limX) x [srcY,
// limY), with samples per pixel placed by supersampleOffset.
func NewGBuffer(srcX, srcY, limX, limY, samples int) *GBuffer {
	n := (limX - srcX) * (limY - srcY) * samples
	gb := &GBuffer{
		SrcX:     srcX,
		SrcY:     srcY,
		LimX:     limX,
		LimY:     limY,
		Samples:  samples,
		InvDepth: make([]float32, n),
		Hits:     make([]VisHit, n),
		Normals:  make([][3]float32, n),
		Albedos:  make([]RGBPower, n),
		sampleX:  make([]int64, samples),
		sampleY:  make([]int64, samples),
	}
	for s := range samples {
		ox, oy := supersampleOffset(s, samples)
		gb.sampleX[s] = int64(math.Round(float64(ox) * rasterSubpixels))
		gb.sampleY[s] = int64(math.Round(float64(oy) * rasterSubpixels))
	}
	for e := range gb.edgeOffsets {
		gb.edgeOffsets[e] = make([]int64, samples)
		gb.planeOffsets[e] = make([]float32, samples)
	}
	gb.Clear()
	return gb
}

// Clear empties every sample, and resets Stats.
func (gb *GBuffer) Clear() {
	gb.Stats = RasterStats{}
	clear(gb.InvDepth)
	for i := range gb.Hits {
		gb.Hits[i] = visHitNone
	}
}

// rasterTri is a triangle set up for rasterization.
type rasterTri struct {
	// Affine functions of raster position (x, y), as {a, b, c} for
	// a*x + b*y + c, giving inverse depth and the numerators of barycentric
	// coordinates 1 and 2 (see the top of this file).
	invDepth, b1, b2 [3]float64

	attrs *RasterAttrs

	// attrs.GeomNormal, flipped if need be to face the camera.
	geomNormal fvec4.T

	hit VisHit
}

// RasterizeTri depth-tests a clip-space triangle against every sample it
// covers, recording hit and the triangle's surface, interpolated from attrs,
// at the samples where it's nearest.  If cullBack is set, triangles facing
// away from the camera (by the winding of their vertices, as in MeshTri) are
// skipped.
func (gb *GBuffer) RasterizeTri(clip [3]fvec4.T, attrs *RasterAttrs, cullBack bool, hit VisHit) {
	gb.Stats.Tris++

	// Trivially reject triangles entirely outside one clip plane.
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

	// Set up the homogeneous interpolation functions.  D's sign is the
	// triangle's facing: raster y points down, so triangles facing the
	// camera wind clockwise on screen, and D < 0.
	var v [3][3]float64
	for i, c := range clip {
		v[i] = [3]float64{float64(c[0]), float64(c[1]), float64(c[3])}
	}
	var e [3][3]float64
	for i := range 3 {
		e[i] = cross3(v[(i+1)%3], v[(i+2)%3])
	}
	d := v[0][0]*e[0][0] + v[0][1]*e[0][1] + v[0][2]*e[0][2]
	if d == 0 {
		// Edge-on to the camera.
		return
	}
	hit.Back = d > 0
	if hit.Back && cullBack {
		return
	}
	tri := rasterTri{attrs: attrs, geomNormal: attrs.GeomNormal, hit: hit}
	if hit.Back {
		tri.geomNormal = fvec4.MulVS(attrs.GeomNormal, -1)
	}
	for k := range 3 {
		tri.invDepth[k] = (e[0][k] + e[1][k] + e[2][k]) / d
		tri.b1[k] = e[1][k] / d
		tri.b2[k] = e[2][k] / d
	}

	// Pass triangles entirely inside all the clip planes straight through.
	// Otherwise, clip to each plane in turn, then fan the remaining convex
	// polygon back into triangles.
	if outside[0]|outside[1]|outside[2] == 0 {
		gb.rasterizeClipped(clip[0], clip[1], clip[2], &tri)
		return
	}
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
		gb.rasterizeClipped(poly[0], poly[i], poly[i+1], &tri)
	}
}

func cross3(a, b [3]float64) [3]float64 {
	return [3]float64{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
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

// rasterizeClipped rasterizes the part of tri covered by the triangle (c0, c1,
// c2), whose vertices are all inside the clip planes.
func (gb *GBuffer) rasterizeClipped(c0, c1, c2 fvec4.T, tri *rasterTri) {
	// Perspective divide, and snap to the subpixel grid for coverage.
	var x, y [3]int64
	for i, c := range [3]fvec4.T{c0, c1, c2} {
		invW := 1 / float64(c[3])
		x[i] = int64(math.Round(float64(c[0]) * invW * rasterSubpixels))
		y[i] = int64(math.Round(float64(c[1]) * invW * rasterSubpixels))
	}

	// Swap two vertices of triangles winding clockwise on screen, so that
	// every triangle's edge functions are positive inside.
	area := (x[1]-x[0])*(y[2]-y[0]) - (y[1]-y[0])*(x[2]-x[0])
	switch {
	case area == 0:
		return
	case area < 0:
		x[1], x[2] = x[2], x[1]
		y[1], y[2] = y[2], y[1]
	}

	// Pixel bounds of the triangle, clamped to the tile.  Pixel p's samples
	// lie in subpixels [p*rasterSubpixels, (p+1)*rasterSubpixels).
	px0 := max(int(min(x[0], x[1], x[2])>>rasterSubpixelBits), gb.SrcX)
	px1 := min(int(max(x[0], x[1], x[2])>>rasterSubpixelBits), gb.LimX-1)
	py0 := max(int(min(y[0], y[1], y[2])>>rasterSubpixelBits), gb.SrcY)
	py1 := min(int(max(y[0], y[1], y[2])>>rasterSubpixelBits), gb.LimY-1)
	if px0 > px1 || py0 > py1 {
		return
	}

	// Edge e runs between the two vertices other than e, and its edge
	// function is proportional to vertex e's (raster space) barycentric
	// coordinate:
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
		for s := range gb.Samples {
			gb.edgeOffsets[e][s] = dx*gb.sampleY[s] - dy*gb.sampleX[s]
		}
	}

	// The interpolation planes, as their value at the origin of pixel (px0,
	// py0) and their steps per pixel and per sample.  The origin value is
	// computed in float64, so that only small offsets from it are in
	// float32.
	n := gb.Samples
	var planeOrigin, planeDX, planeDY [3]float32
	for k, p := range [3]*[3]float64{&tri.invDepth, &tri.b1, &tri.b2} {
		planeOrigin[k] = float32(p[0]*float64(px0) + p[1]*float64(py0) + p[2])
		planeDX[k] = float32(p[0])
		planeDY[k] = float32(p[1])
		for s := range n {
			gb.planeOffsets[k][s] = planeDX[k]*float32(gb.sampleX[s])/rasterSubpixels + planeDY[k]*float32(gb.sampleY[s])/rasterSubpixels
		}
	}

	gb.Stats.TrisSetUp++
	gb.Stats.SamplesTested += int64((px1-px0+1)*(py1-py0+1)) * int64(n)
	var samplesHit int64

	// Attributes are interpolated as offsets from vertex 0's, which is exact
	// when the vertices share a value (see MeshInstance.triContact).
	attrs := tri.attrs
	n0 := attrs.Normals[0]
	dn1 := fvec4.SubVV(attrs.Normals[1], n0)
	dn2 := fvec4.SubVV(attrs.Normals[2], n0)
	gn := tri.geomNormal
	uv0 := attrs.UVs[0]
	duv1 := fvec2.T{attrs.UVs[1][0] - uv0[0], attrs.UVs[1][1] - uv0[1]}
	duv2 := fvec2.T{attrs.UVs[2][0] - uv0[0], attrs.UVs[2][1] - uv0[1]}

	off0 := gb.edgeOffsets[0][:n]
	off1 := gb.edgeOffsets[1][:n]
	off2 := gb.edgeOffsets[2][:n]
	offZ := gb.planeOffsets[0][:n]
	offB1 := gb.planeOffsets[1][:n]
	offB2 := gb.planeOffsets[2][:n]
	tileW := gb.LimX - gb.SrcX
	for py := py0; py <= py1; py++ {
		e0, e1, e2 := rowE[0], rowE[1], rowE[2]
		dy := float32(py - py0)
		rowZ := planeOrigin[0] + planeDY[0]*dy
		rowB1 := planeOrigin[1] + planeDY[1]*dy
		rowB2 := planeOrigin[2] + planeDY[2]*dy
		i := ((py-gb.SrcY)*tileW + (px0 - gb.SrcX)) * n
		for px := px0; px <= px1; px++ {
			dx := float32(px - px0)
			pixelZ := rowZ + planeDX[0]*dx
			pixelB1 := rowB1 + planeDX[1]*dx
			pixelB2 := rowB2 + planeDX[2]*dx
			depths := gb.InvDepth[i : i+n]
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
				samplesHit++

				// Perspective-correct barycentric coordinates.
				invD := 1 / d
				b1 := (pixelB1 + offB1[s]) * invD
				b2 := (pixelB2 + offB2[s]) * invD

				// Like RayContact, flip the shading normal to agree with
				// the geometric normal, which faces the camera.
				nx := n0[0] + b1*dn1[0] + b2*dn2[0]
				ny := n0[1] + b1*dn1[1] + b2*dn2[1]
				nz := n0[2] + b1*dn1[2] + b2*dn2[2]
				if nx*gn[0]+ny*gn[1]+nz*gn[2] < 0 {
					nx, ny, nz = -nx, -ny, -nz
				}

				uv := fvec2.T{
					uv0[0] + b1*duv1[0] + b2*duv2[0],
					uv0[1] + b1*duv1[1] + b2*duv2[1],
				}

				gb.Hits[i+s] = tri.hit
				gb.Normals[i+s] = [3]float32{nx, ny, nz}
				gb.Albedos[i+s] = sampleDiffuse(attrs.Diffuse, uv)
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
	gb.Stats.SamplesHit += samplesHit
}
