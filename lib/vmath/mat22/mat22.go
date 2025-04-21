package mat22

import (
	"math"
	"shellworld/lib/vmath/vec2"
)

type T [4]float64

func Identity() T {
	return T{1.0, 0.0, 0.0, 1.0}
}

func Rotate(theta float64) T {
	c := math.Cos(theta)
	s := math.Sin(theta)
	return T{c, -s, s, c}
}

func MulMM(a, b T) T {
	result := T{}
	result[0*2+0] = a[0*2+0]*b[0*2+0] + a[0*2+1]*b[1*2+0]
	result[0*2+1] = a[0*2+0]*b[0*2+1] + a[0*2+1]*b[1*2+1]
	result[1*2+0] = a[1*2+0]*b[0*2+0] + a[1*2+1]*b[1*2+0]
	result[1*2+1] = a[1*2+0]*b[0*2+1] + a[1*2+1]*b[1*2+1]
	return result
}

func MulMV(a T, b vec2.T) vec2.T {
	return vec2.T{
		a[0*2+0]*b[0] + a[0*2+1]*b[1],
		a[1*2+0]*b[0] + a[1*2+1]*b[1],
	}
}
