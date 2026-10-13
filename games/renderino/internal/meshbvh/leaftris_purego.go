//go:build !goexperiment.simd || !amd64

package meshbvh

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// rayIntersect tests a ray against every lane of the leaf, giving for each
// lane exactly the result that Triangle.RayIntersect would for its triangle.  It returns a bitmask with bit i set if lane i was hit with
// queryLo < t < queryHi, and stores each hit lane's ray parameter and
// barycentric coordinates (b1, b2) into t, b1 and b2.
//
// This build tests the lanes one at a time; the SIMD build tests them all at
// once.
func (l *leafTris) rayIntersect(point, slope fvec4.T, queryLo, queryHi float32, t, b1, b2 *[LeafSize]float32) uint8 {
	return l.rayIntersectLanes(point, slope, queryLo, queryHi, t, b1, b2)
}
