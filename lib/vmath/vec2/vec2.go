package vec2

import (
	"math"
	"math/rand/v2"
)

type T [2]float64

func (v T) Normalize() T {
	mag := math.Sqrt(v[0]*v[0] + v[1]*v[1])
	return T{v[0] / mag, v[1] / mag}
}

func (v T) Norm() float64 {
	return math.Sqrt(v[0]*v[0] + v[1]*v[1])
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

func MulSV(s float64, a T) T {
	return T{
		s * a[0],
		s * a[1],
	}
}

func Dot(a, b T) float64 {
	return a[0]*b[0] + a[1]*b[1]
}

func UniformDiskDistribution(rng *rand.Rand, radius float64) T {
	result := T{}
	for {
		result[0] = 2 * radius * (rng.Float64() - 0.5)
		result[1] = 2 * radius * (rng.Float64() - 0.5)
		normSquared := result[0]*result[0] + result[1]*result[1]
		if normSquared <= radius*radius && normSquared != 0.0 {
			break
		}
	}
	return result
}
