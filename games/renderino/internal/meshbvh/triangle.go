package meshbvh

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// Triangle is a triangle prepared for Möller-Trumbore intersection: its first
// vertex, and the edges from it to the other two vertices.  All vectors have
// w = 0.
type Triangle struct {
	V0, E1, E2 fvec4.T
}

// NewTriangle makes the Triangle with vertices v0, v1 and v2.
func NewTriangle(v0, v1, v2 fvec4.T) Triangle {
	return Triangle{
		V0: v0,
		E1: fvec4.SubVV(v1, v0),
		E2: fvec4.SubVV(v2, v0),
	}
}

// bounds returns the triangle's bounds.
func (tri *Triangle) bounds() aabb {
	lo, hi := tri.V0, tri.V0
	for _, e := range [2]fvec4.T{tri.E1, tri.E2} {
		for k := range 3 {
			v := tri.V0[k] + e[k]
			lo[k] = min(lo[k], v)
			hi[k] = max(hi[k], v)
		}
	}
	return aabb{Lo: lo, Hi: hi}
}

// RayIntersect tests a ray against the triangle using the Möller-Trumbore
// algorithm.  On a hit with queryLo < t < queryHi, it returns the ray
// parameter t and the barycentric coordinates (b1, b2) of the hit point with
// respect to vertices 1 and 2.
//
// It's written with scalar locals rather than fvec4.T temporaries: the
// compiler spills fvec4.T values to the stack as separate 4-byte stores and
// reloads them with one 16-byte load, which stalls on store forwarding.
func (tri *Triangle) RayIntersect(point, slope fvec4.T, queryLo, queryHi float32) (t, b1, b2 float32, ok bool) {
	e1x, e1y, e1z := tri.E1[0], tri.E1[1], tri.E1[2]
	e2x, e2y, e2z := tri.E2[0], tri.E2[1], tri.E2[2]
	dx, dy, dz := slope[0], slope[1], slope[2]

	px := dy*e2z - dz*e2y
	py := dz*e2x - dx*e2z
	pz := dx*e2y - dy*e2x
	det := e1x*px + e1y*py + e1z*pz
	if det == 0 {
		// Ray is parallel to the triangle.
		return 0, 0, 0, false
	}
	invDet := 1 / det

	sx := point[0] - tri.V0[0]
	sy := point[1] - tri.V0[1]
	sz := point[2] - tri.V0[2]
	b1 = (sx*px + sy*py + sz*pz) * invDet
	if b1 < 0 || b1 > 1 {
		return 0, 0, 0, false
	}

	qx := sy*e1z - sz*e1y
	qy := sz*e1x - sx*e1z
	qz := sx*e1y - sy*e1x
	b2 = (dx*qx + dy*qy + dz*qz) * invDet
	if b2 < 0 || b1+b2 > 1 {
		return 0, 0, 0, false
	}

	t = (e2x*qx + e2y*qy + e2z*qz) * invDet
	if t <= queryLo || t >= queryHi {
		return 0, 0, 0, false
	}

	return t, b1, b2, true
}

// leafTris holds a leaf's triangles in struct-of-arrays form -- one lane per
// triangle -- so that a ray can be tested against all of them at once.  Each
// lane holds its triangle's first vertex and the edges from it to the other
// two, as in Triangle.
//
// Unused lanes hold a degenerate triangle at the origin with zero-length
// edges, which a ray can never hit: Möller-Trumbore's determinant is zero,
// and the barycentric coordinates come out NaN.
type leafTris struct {
	V0X, V0Y, V0Z [LeafSize]float32
	E1X, E1Y, E1Z [LeafSize]float32
	E2X, E2Y, E2Z [LeafSize]float32

	// The ref of each lane's triangle.  Only the first N are valid.
	Refs [LeafSize]int32
	N    int32
}

// makeLeaf builds a leaf holding the triangles of the elements located by s,
// with bounds that are the union of theirs, and appends their triangles to
// t.leaves.
func (t *Tree) makeLeaf(s span) *Node {
	elements := t.elements[s.first : s.first+s.count]

	t.leaves = append(t.leaves, leafTris{N: int32(len(elements))})
	l := &t.leaves[len(t.leaves)-1]

	lo, hi := elements[0].Bounds.Lo, elements[0].Bounds.Hi
	for lane, e := range elements {
		for k := range 3 {
			lo[k] = min(lo[k], e.Bounds.Lo[k])
			hi[k] = max(hi[k], e.Bounds.Hi[k])
		}

		tri := &t.triangles[e.Ref]
		l.V0X[lane], l.V0Y[lane], l.V0Z[lane] = tri.V0[0], tri.V0[1], tri.V0[2]
		l.E1X[lane], l.E1Y[lane], l.E1Z[lane] = tri.E1[0], tri.E1[1], tri.E1[2]
		l.E2X[lane], l.E2Y[lane], l.E2Z[lane] = tri.E2[0], tri.E2[1], tri.E2[2]
		l.Refs[lane] = int32(e.Ref)
	}

	node := t.getFreeNode()
	node.Type = NodeTypeConcrete
	node.Lo, node.Hi = lo, hi
	node.Ref = len(t.leaves) - 1
	return node
}

// rayIntersectLanes is leafTris.rayIntersect done one lane at a time, with
// Triangle.RayIntersect.
func (l *leafTris) rayIntersectLanes(point, slope fvec4.T, queryLo, queryHi float32, t, b1, b2 *[LeafSize]float32) uint8 {
	var mask uint8
	for i := range int(l.N) {
		tri := Triangle{
			V0: fvec4.T{l.V0X[i], l.V0Y[i], l.V0Z[i], 0},
			E1: fvec4.T{l.E1X[i], l.E1Y[i], l.E1Z[i], 0},
			E2: fvec4.T{l.E2X[i], l.E2Y[i], l.E2Z[i], 0},
		}
		var ok bool
		t[i], b1[i], b2[i], ok = tri.RayIntersect(point, slope, queryLo, queryHi)
		if ok {
			mask |= 1 << i
		}
	}
	return mask
}
