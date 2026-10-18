//go:build !goexperiment.simd

package fmat44

import "github.com/ahmedtd/gogames/lib/vmath/fvec4"

func MulMV(a T, b fvec4.T) fvec4.T {
	return fvec4.T{
		a[0]*b[0] + a[1]*b[1] + a[2]*b[2] + a[3]*b[3],
		a[4]*b[0] + a[5]*b[1] + a[6]*b[2] + a[7]*b[3],
		a[8]*b[0] + a[9]*b[1] + a[10]*b[2] + a[11]*b[3],
		a[12]*b[0] + a[13]*b[1] + a[14]*b[2] + a[15]*b[3],
	}
}

// MulMV2 computes a*b and a*c.
func MulMV2(a T, b, c fvec4.T) (fvec4.T, fvec4.T) {
	return MulMV(a, b), MulMV(a, c)
}
