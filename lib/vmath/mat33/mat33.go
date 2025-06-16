package mat33

import (
	"math"

	"shellworld/lib/vmath/mat22"
	"shellworld/lib/vmath/vec2"
	"shellworld/lib/vmath/vec3"
)

type T [9]float64

func Identity() T {
	return T{1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0}
}

func Transform2(scale float64, orientation mat22.T, offset vec2.T) T {
	return T{
		orientation[0] * scale, orientation[1] * scale, offset[0],
		orientation[2] * scale, orientation[3] * scale, offset[1],
		0, 0, 1,
	}
}

func Rotate2(angle float64) T {
	return T{
		math.Cos(angle), math.Sin(angle), 0.0,
		-math.Sin(angle), math.Cos(angle), 0.0,
		0.0, 0.0, 1.0,
	}
}

func Scale(s float64) T {
	return T{
		s, 0.0, 0.0,
		0.0, s, 0.0,
		0.0, 0.0, 1.0,
	}
}

func Scale2(sx, sy float64) T {
	return T{
		sx, 0.0, 0.0,
		0.0, sy, 0.0,
		0.0, 0.0, 1.0,
	}
}

func Translate2(t vec2.T) T {
	return T{
		1.0, 0.0, t[0],
		0.0, 1.0, t[1],
		0.0, 0.0, 1.0,
	}
}

func MulMM(a, b T) T {
	result := T{}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				result[i*3+j] += a[i*3+k] * b[k*3+j]
			}
		}
	}
	return result
}

func MulMV(a T, b vec3.T) vec3.T {
	return vec3.T{
		a[0]*b[0] + a[1]*b[1] + a[2]*b[2],
		a[3]*b[0] + a[4]*b[1] + a[5]*b[2],
		a[6]*b[0] + a[7]*b[1] + a[8]*b[2],
	}
}

func MulMV2(a T, b vec2.T) vec2.T {
	return vec2.T{
		a[0]*b[0] + a[1]*b[1] + a[2],
		a[3]*b[0] + a[4]*b[1] + a[5],
	}
}

func Transpose(m T) T {
	transpose := T{}
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			transpose[c*3+r] = m[r*3+c]
		}
	}
	return transpose
}

func rowEchelonInplace(m, a *T) {
	for k := 0; k < 3; k++ {
		// Select the row below row k with the best pivot.
		maxRow := k
		for i := k; i < 3; i++ {
			if math.Abs(m[i*3+k]) > math.Abs(m[maxRow*3+k]) {
				maxRow = i
			}
		}

		// Swap selected row to current row.
		for i := 0; i < 3; i++ {
			m[k*3+i], m[maxRow*3+i] = m[maxRow*3+i], m[k*3+i]
			a[k*3+i], a[maxRow*3+i] = a[maxRow*3+i], a[k*3+i]
		}

		// Now the pivot element is at m[k, k].
		pivot := m[k*3+k]
		for r := k + 1; r < 3; r++ {
			scale := m[r*3+k] / pivot
			for c := k + 1; c < 3; c++ {
				m[r*3+c] -= m[k*3+c] * scale
			}
			for c := 0; c < 3; c++ {
				a[r*3+c] -= a[k*3+c] * scale
			}
			m[r*3+k] = 0.0
		}
	}

}

func backsubInplace(m, a *T) {
	for k := 3 - 1; k > 0; k-- {
		// m[k,k] is the pivot

		// Nullify all entries above the pivot element.
		for r := 0; r < k; r++ {
			scale := m[r*3+k] / m[k*3+k]

			m[r*3+k] = 0
			for c := k + 1; c < 3; c++ {
				m[r*3+c] -= m[k*3+c] * scale
			}

			// Mirror the action in the augmented matrix.
			for c := 0; c < 3; c++ {
				a[r*3+c] -= a[k*3+c] * scale
			}
		}
	}

	// Now we simply need to divide each row by its pivot.
	for k := 0; k < 3; k++ {
		for c := k + 1; c < 3; c++ {
			m[k*3+c] /= m[k*3+k]
		}
		for c := 0; c < 3; c++ {
			a[k*3+c] /= m[k*3+k]
		}
		m[k*3+k] = 1
	}
}

func SolveInplace(m, a *T) {
	rowEchelonInplace(m, a)
	backsubInplace(m, a)
}

func Inverse(m T) T {
	a := Identity()
	SolveInplace(&m, &a)
	return a
}
