//go:build !goexperiment.simd || !amd64

package main

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// RayIntersect tests a model-space ray against every lane of the packet,
// giving for each lane exactly the result that MeshTri.RayIntersect would for
// its triangle.  It returns a bitmask with bit i set if lane i was hit with
// queryLo < t < queryHi, and stores each hit lane's ray parameter and
// barycentric coordinates (b1, b2) into t, b1 and b2.
//
// This build tests the lanes one at a time; the SIMD build tests them all at
// once.
func (p *TriPacket) RayIntersect(point, slope fvec4.T, queryLo, queryHi float32, t, b1, b2 *[triPacketSize]float32) uint8 {
	return p.rayIntersectLanes(point, slope, queryLo, queryHi, t, b1, b2)
}
