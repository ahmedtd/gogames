package main

import (
	"github.com/ahmedtd/gogames/games/renderino/internal/bvh"
	"github.com/ahmedtd/gogames/lib/graphics/renderable3"
	"github.com/ahmedtd/gogames/lib/vmath/fmat44"
	"github.com/ahmedtd/gogames/lib/vmath/fvec2"
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"

	"github.com/barnex/fmath"
)

// MeshModel is a triangle mesh prepared for raytracing.  Triangles are stored
// in model space, along with a BVH over their bounds.  A MeshModel is built
// once, and can be shared by any number of MeshInstances.
type MeshModel struct {
	Tris []MeshTri
	Tree *bvh.Tree

	// Model-space bounds of all triangles.
	Lo, Hi fvec4.T

	Diffuse *renderable3.UncompressedImage
}

// MeshTri is a single model-space triangle.  All vectors have w = 0.
type MeshTri struct {
	// The first vertex, and the edges from it to the other two vertices,
	// precomputed for Möller-Trumbore intersection.
	V0, E1, E2 fvec4.T

	// Per-vertex normals and texture coordinates.
	Ns  [3]fvec4.T
	UVs [3]fvec2.T
}

// NewMeshModel converts a renderable3 mesh into a MeshModel, building its BVH.
func NewMeshModel(mesh *renderable3.T) *MeshModel {
	m := &MeshModel{
		Tris:    make([]MeshTri, len(mesh.Tris)),
		Tree:    &bvh.Tree{},
		Diffuse: mesh.Diffuse,
	}

	nodes := make([]*bvh.Node, len(mesh.Tris))
	for i, tri := range mesh.Tris {
		var vs [3]fvec4.T
		for j := range 3 {
			v := mesh.Vertices[tri.VertexIndices[j]]
			vs[j] = fvec4.T{v[0], v[1], v[2], 0}
		}

		mt := &m.Tris[i]
		mt.V0 = vs[0]
		mt.E1 = fvec4.SubVV(vs[1], vs[0])
		mt.E2 = fvec4.SubVV(vs[2], vs[0])

		for j := range 3 {
			if len(mesh.Normals) != 0 {
				n := mesh.Normals[tri.NormalIndices[j]]
				mt.Ns[j] = fvec4.T{n[0], n[1], n[2], 0}
			} else {
				mt.Ns[j] = fvec4.Normalize(fvec4.CProd3(mt.E1, mt.E2))
			}
			if len(mesh.TextureCoords) != 0 {
				mt.UVs[j] = mesh.TextureCoords[tri.TextureIndices[j]]
			}
		}

		lo, hi := vs[0], vs[0]
		for _, v := range vs[1:] {
			for k := range 3 {
				lo[k] = min(lo[k], v[k])
				hi[k] = max(hi[k], v[k])
			}
		}

		node := m.Tree.GetFreeNode()
		node.Reset(bvh.NodeTypeConcrete, lo, hi, i)
		nodes[i] = node

		if i == 0 {
			m.Lo, m.Hi = lo, hi
		} else {
			for k := range 3 {
				m.Lo[k] = min(m.Lo[k], lo[k])
				m.Hi[k] = max(m.Hi[k], hi[k])
			}
		}
	}

	m.Tree.BuildFromElements(nodes)

	return m
}

// RayIntersect tests a model-space ray against the triangle using the
// Möller-Trumbore algorithm.  On a hit with queryLo < t < queryHi, it returns
// the ray parameter t and the barycentric coordinates (b1, b2) of the hit
// point with respect to vertices 1 and 2.
func (tri *MeshTri) RayIntersect(point, slope fvec4.T, queryLo, queryHi float32) (t, b1, b2 float32, ok bool) {
	p := fvec4.CProd3(slope, tri.E2)
	det := dot3(tri.E1, p)
	if det == 0 {
		// Ray is parallel to the triangle.
		return 0, 0, 0, false
	}
	invDet := 1 / det

	s := fvec4.T{point[0] - tri.V0[0], point[1] - tri.V0[1], point[2] - tri.V0[2], 0}
	b1 = dot3(s, p) * invDet
	if b1 < 0 || b1 > 1 {
		return 0, 0, 0, false
	}

	q := fvec4.CProd3(s, tri.E1)
	b2 = dot3(slope, q) * invDet
	if b2 < 0 || b1+b2 > 1 {
		return 0, 0, 0, false
	}

	t = dot3(tri.E2, q) * invDet
	if t <= queryLo || t >= queryHi {
		return 0, 0, 0, false
	}

	return t, b1, b2, true
}

// MeshInstance places a MeshModel in the world with an affine transform.
type MeshInstance struct {
	Model *MeshModel

	modelToWorld fmat44.T
	worldToModel fmat44.T

	// World-space bounds of the transformed model.
	Lo, Hi fvec4.T
}

func NewMeshInstance(model *MeshModel, modelToWorld fmat44.T) MeshInstance {
	m := MeshInstance{Model: model}
	m.SetModelToWorld(modelToWorld)
	return m
}

func (m *MeshInstance) ModelToWorld() fmat44.T {
	return m.modelToWorld
}

// SetModelToWorld updates the instance's transform, recomputing its inverse
// and world-space bounds.
func (m *MeshInstance) SetModelToWorld(modelToWorld fmat44.T) {
	m.modelToWorld = modelToWorld
	m.worldToModel = fmat44.Inverse(modelToWorld)

	// Transform all 8 corners of the model-space bounds, and take their
	// world-space bounds.
	for i := range 8 {
		corner := fvec4.T{m.Model.Lo[0], m.Model.Lo[1], m.Model.Lo[2], 1}
		if i&1 != 0 {
			corner[0] = m.Model.Hi[0]
		}
		if i&2 != 0 {
			corner[1] = m.Model.Hi[1]
		}
		if i&4 != 0 {
			corner[2] = m.Model.Hi[2]
		}
		w := fmat44.MulMV(modelToWorld, corner)
		w[3] = 0

		if i == 0 {
			m.Lo, m.Hi = w, w
		} else {
			for k := range 3 {
				m.Lo[k] = min(m.Lo[k], w[k])
				m.Hi[k] = max(m.Hi[k], w[k])
			}
		}
	}
}

// RayContact finds the nearest triangle of the instance hit by a world-space
// ray.
//
// The ray is carried into model space without renormalizing its slope, so the
// ray parameter t is the same in both spaces and can be compared directly
// against contacts with other scene objects.
func (m *MeshInstance) RayContact(ray Ray, queryLo, queryHi float32) Contact {
	modelPoint := fmat44.MulMV(m.worldToModel, fvec4.T{ray.Point[0], ray.Point[1], ray.Point[2], 1})
	modelPoint[3] = 0
	modelSlope := fmat44.MulMV(m.worldToModel, fvec4.T{ray.Slope[0], ray.Slope[1], ray.Slope[2], 0})

	tris := m.Model.Tris
	hitRef := -1
	var hitT, hitB1, hitB2 float32
	hitT = queryHi
	m.Model.Tree.RayTest(modelPoint, modelSlope, func(ref int) {
		t, b1, b2, ok := tris[ref].RayIntersect(modelPoint, modelSlope, queryLo, hitT)
		if ok {
			hitRef, hitT, hitB1, hitB2 = ref, t, b1, b2
		}
	})

	if hitRef == -1 {
		return Contact{Mode: ContactModeMiss}
	}

	tri := &tris[hitRef]
	b0 := 1 - hitB1 - hitB2

	modelN := fvec4.T{
		b0*tri.Ns[0][0] + hitB1*tri.Ns[1][0] + hitB2*tri.Ns[2][0],
		b0*tri.Ns[0][1] + hitB1*tri.Ns[1][1] + hitB2*tri.Ns[2][1],
		b0*tri.Ns[0][2] + hitB1*tri.Ns[1][2] + hitB2*tri.Ns[2][2],
		0,
	}
	n := fvec4.Normalize(m.normalToWorld(modelN))

	// Like AABB, the contact normal always faces back towards the ray origin.
	// Use the geometric normal to decide whether we entered or exited the
	// surface, since interpolated normals can disagree near silhouettes.
	contact := Contact{
		Mode:    ContactModeEnter,
		ExitRay: ray,
		T:       hitT,
		P:       ray.Eval(hitT),
		N:       n,
	}
	contact.P[3] = 0

	geomN := m.normalToWorld(fvec4.CProd3(tri.E1, tri.E2))
	if dot3(geomN, ray.Slope) > 0 {
		contact.Mode = ContactModeExit
		geomN = fvec4.MulVS(geomN, -1)
	}
	if dot3(contact.N, geomN) < 0 {
		contact.N = fvec4.MulVS(contact.N, -1)
	}

	uv := fvec2.T{
		b0*tri.UVs[0][0] + hitB1*tri.UVs[1][0] + hitB2*tri.UVs[2][0],
		b0*tri.UVs[0][1] + hitB1*tri.UVs[1][1] + hitB2*tri.UVs[2][1],
	}
	contact.Albedo = sampleDiffuse(m.Model.Diffuse, uv)

	return contact
}

// normalToWorld carries a model-space normal into world space using the
// inverse transpose of the model-to-world transform.  The result is not
// normalized.
func (m *MeshInstance) normalToWorld(n fvec4.T) fvec4.T {
	w := &m.worldToModel
	return fvec4.T{
		w[0]*n[0] + w[4]*n[1] + w[8]*n[2],
		w[1]*n[0] + w[5]*n[1] + w[9]*n[2],
		w[2]*n[0] + w[6]*n[1] + w[10]*n[2],
		0,
	}
}

// sampleDiffuse does a nearest-neighbor lookup of an OBJ-convention texture
// coordinate (v = 0 is the bottom row of the image), repeating outside [0, 1).
// A nil image is treated as white.
func sampleDiffuse(img *renderable3.UncompressedImage, uv fvec2.T) RGBPower {
	if img == nil {
		return RGBPower{1, 1, 1}
	}

	u := uv[0] - fmath.Floor(uv[0])
	v := 1 - (uv[1] - fmath.Floor(uv[1]))

	x := min(int(u*float32(img.Width)), img.Width-1)
	y := min(int(v*float32(img.Height)), img.Height-1)

	i := (y*img.Width + x) * 4
	return RGBPower{img.Samples[i+0], img.Samples[i+1], img.Samples[i+2]}
}

func dot3(a, b fvec4.T) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

// translateMat44 builds an affine transform that translates by t.
func translateMat44(t fvec4.T) fmat44.T {
	return fmat44.T{
		1, 0, 0, t[0],
		0, 1, 0, t[1],
		0, 0, 1, t[2],
		0, 0, 0, 1,
	}
}

// scaleMat44 builds an affine transform that scales uniformly by s.
func scaleMat44(s float32) fmat44.T {
	return fmat44.T{
		s, 0, 0, 0,
		0, s, 0, 0,
		0, 0, s, 0,
		0, 0, 0, 1,
	}
}
