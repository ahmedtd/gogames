//go:build goexperiment.simd && amd64

package fmat33

import "github.com/ahmedtd/gogames/lib/vmath/fvec3"

func MulMV(a T, b fvec3.T) fvec3.T {
	return fvec3.T{
		a[0]*b[0] + a[1]*b[1] + a[2]*b[2],
		a[3]*b[0] + a[4]*b[1] + a[5]*b[2],
		a[6]*b[0] + a[7]*b[1] + a[8]*b[2],
	}
}
