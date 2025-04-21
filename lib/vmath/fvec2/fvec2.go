package fvec2

import (
	"math/rand/v2"

	"github.com/barnex/fmath"
)

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

func UniformDiskDistribution(rng *rand.Rand, radius float32) T {
	result := T{}
	for {
		result[0] = 2 * radius * (rng.Float32() - 0.5)
		result[1] = 2 * radius * (rng.Float32() - 0.5)
		normSquared := result[0]*result[0] + result[1]*result[1]
		if normSquared <= radius*radius && normSquared != 0.0 {
			break
		}
	}
	return result
}
