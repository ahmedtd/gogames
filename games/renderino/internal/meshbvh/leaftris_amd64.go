//go:build goexperiment.simd && amd64

package meshbvh

import (
	"simd/archsimd"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// rayIntersect tests a ray against every lane of the leaf at once, using the
// same Möller-Trumbore arithmetic as Triangle.RayIntersect, operation for
// operation, so each lane gives exactly the result that
// Triangle.RayIntersect would for its triangle.  It returns a bitmask with bit
// i set if lane i was hit with queryLo < t < queryHi, and stores each lane's
// ray parameter and barycentric coordinates (b1, b2) into t, b1 and b2; these
// are only meaningful for lanes that were hit.
func (l *leafTris) rayIntersect(point, slope fvec4.T, queryLo, queryHi float32, t, b1, b2 *[LeafSize]float32) uint8 {
	e1x := archsimd.LoadFloat32x4(&l.E1X)
	e1y := archsimd.LoadFloat32x4(&l.E1Y)
	e1z := archsimd.LoadFloat32x4(&l.E1Z)
	e2x := archsimd.LoadFloat32x4(&l.E2X)
	e2y := archsimd.LoadFloat32x4(&l.E2Y)
	e2z := archsimd.LoadFloat32x4(&l.E2Z)
	dx := archsimd.BroadcastFloat32x4(slope[0])
	dy := archsimd.BroadcastFloat32x4(slope[1])
	dz := archsimd.BroadcastFloat32x4(slope[2])

	px := dy.Mul(e2z).Sub(dz.Mul(e2y))
	py := dz.Mul(e2x).Sub(dx.Mul(e2z))
	pz := dx.Mul(e2y).Sub(dy.Mul(e2x))
	det := e1x.Mul(px).Add(e1y.Mul(py)).Add(e1z.Mul(pz))
	// Where det is zero (the ray is parallel to the triangle), invDet is
	// infinite, and b1 below is either infinite or NaN, failing its range
	// check -- just as Triangle.RayIntersect rejects it up front.
	invDet := archsimd.BroadcastFloat32x4(1).Div(det)

	sx := archsimd.BroadcastFloat32x4(point[0]).Sub(archsimd.LoadFloat32x4(&l.V0X))
	sy := archsimd.BroadcastFloat32x4(point[1]).Sub(archsimd.LoadFloat32x4(&l.V0Y))
	sz := archsimd.BroadcastFloat32x4(point[2]).Sub(archsimd.LoadFloat32x4(&l.V0Z))
	b1v := sx.Mul(px).Add(sy.Mul(py)).Add(sz.Mul(pz)).Mul(invDet)

	qx := sy.Mul(e1z).Sub(sz.Mul(e1y))
	qy := sz.Mul(e1x).Sub(sx.Mul(e1z))
	qz := sx.Mul(e1y).Sub(sy.Mul(e1x))
	b2v := dx.Mul(qx).Add(dy.Mul(qy)).Add(dz.Mul(qz)).Mul(invDet)

	tv := e2x.Mul(qx).Add(e2y.Mul(qy)).Add(e2z.Mul(qz)).Mul(invDet)

	zero := archsimd.BroadcastFloat32x4(0)
	one := archsimd.BroadcastFloat32x4(1)
	hit := b1v.GreaterEqual(zero).
		And(b1v.LessEqual(one)).
		And(b2v.GreaterEqual(zero)).
		And(b1v.Add(b2v).LessEqual(one)).
		And(tv.Greater(archsimd.BroadcastFloat32x4(queryLo))).
		And(tv.Less(archsimd.BroadcastFloat32x4(queryHi)))

	tv.Store(t)
	b1v.Store(b1)
	b2v.Store(b2)
	return hit.ToBits()
}
