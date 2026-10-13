package meshbvh

import (
	"math"
	"math/bits"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// Hit is a ray's hit on one of a tree's triangles.
type Hit struct {
	// The triangle's ref (see AddTriangle).
	Ref int

	// The ray parameter of the hit point, and its barycentric coordinates
	// with respect to the triangle's vertices 1 and 2.
	T, B1, B2 float32
}

// RayTest finds the nearest triangle hit by a ray (origin point, direction
// slope) with tLo < t < tHi.  It reports whether there was one.
//
// The traversal's work stack is kept in stack, which may be nil to use a
// temporary one.  The work done is added into stats, which may be nil.
func (t *Tree) RayTest(point, slope fvec4.T, tLo, tHi float32, stack *Stack, stats *Stats) (Hit, bool) {
	hit := Hit{Ref: -1, T: tHi}
	var triTests int64
	t.rayTest(point, slope, tLo, tHi, stack, stats, true, func(leaf int) float32 {
		l := &t.leaves[leaf]
		triTests += int64(l.N)
		var ts, b1, b2 [LeafSize]float32
		hits := l.rayIntersect(point, slope, tLo, hit.T, &ts, &b1, &b2)
		for hits != 0 {
			i := bits.TrailingZeros8(hits)
			hits &= hits - 1
			if ts[i] < hit.T {
				hit = Hit{Ref: int(l.Refs[i]), T: ts[i], B1: b1[i], B2: b2[i]}
			}
		}
		return hit.T
	})
	if stats != nil {
		stats.TriTests += triTests
	}
	return hit, hit.Ref != -1
}

// RayOccluded reports whether a ray (origin point, direction slope) hits any
// triangle with tLo < t < tHi.  Unlike RayTest, it stops at the first hit
// found rather than searching for the nearest, so it's suited to shadow rays.
//
// The traversal's work stack is kept in stack, which may be nil to use a
// temporary one.  The work done is added into stats, which may be nil.
func (t *Tree) RayOccluded(point, slope fvec4.T, tLo, tHi float32, stack *Stack, stats *Stats) bool {
	occluded := false
	var triTests int64
	t.rayTest(point, slope, tLo, tHi, stack, stats, false, func(leaf int) float32 {
		l := &t.leaves[leaf]
		triTests += int64(l.N)
		var ts, b1, b2 [LeafSize]float32
		if l.rayIntersect(point, slope, tLo, tHi, &ts, &b1, &b2) != 0 {
			occluded = true
			// A bound below tLo prunes the rest of the traversal.
			return float32(math.Inf(-1))
		}
		return tHi
	})
	if stats != nil {
		stats.TriTests += triTests
	}
	return occluded
}
