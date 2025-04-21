package fvec2

import "github.com/barnex/fmath"

type T [2]float32

func (v T) Normalize() T {
	mag := fmath.Sqrt(v[0]*v[0] + v[1]*v[1])
	return T{v[0] / mag, v[1] / mag}
}

func (v T) Norm() float32 {
	return fmath.Sqrt(v[0]*v[0] + v[1]*v[1])
}

func Add(a, b T) T {
	return T{
		a[0] + b[0],
		a[1] + b[1],
	}
}

func Sub(a, b T) T {
	return T{
		a[0] - b[0],
		a[1] - b[1],
	}
}

func MulSV(s float32, a T) T {
	return T{
		s * a[0],
		s * a[1],
	}
}

func Dot(a, b T) float32 {
	return a[0]*b[0] + a[1]*b[1]
}
