//go:build goexperiment.simd && amd64

package fmat44

import (
	"simd/archsimd"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// MulMV computes a*b.  Each row of a occupies exactly one 128-bit SIMD lane,
// so we load all four rows and b into registers, multiply row-wise, and use
// two rounds of horizontal adds to reduce all four row.b dot products down to
// a single result register, rather than reducing each row individually.
func MulMV(a T, b fvec4.T) fvec4.T {
	bv := archsimd.LoadFloat32x4((*[4]float32)(&b))

	row0 := archsimd.LoadFloat32x4((*[4]float32)(a[0:4]))
	row1 := archsimd.LoadFloat32x4((*[4]float32)(a[4:8]))
	row2 := archsimd.LoadFloat32x4((*[4]float32)(a[8:12]))
	row3 := archsimd.LoadFloat32x4((*[4]float32)(a[12:16]))

	p0 := row0.Mul(bv)
	p1 := row1.Mul(bv)
	p2 := row2.Mul(bv)
	p3 := row3.Mul(bv)

	// lo01 = [p0[0]+p0[1], p0[2]+p0[3], p1[0]+p1[1], p1[2]+p1[3]]
	lo01 := p0.AddPairs(p1)
	// lo23 = [p2[0]+p2[1], p2[2]+p2[3], p3[0]+p3[1], p3[2]+p3[3]]
	lo23 := p2.AddPairs(p3)
	// result = [dot(row0, b), dot(row1, b), dot(row2, b), dot(row3, b)]
	result := lo01.AddPairs(lo23)

	out := fvec4.T{}
	result.Store((*[4]float32)(&out))
	return out
}
