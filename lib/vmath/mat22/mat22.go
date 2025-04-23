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

func rowEchelonInplace(m, a *T) {
	for k := 0; k < 2; k++ {
		// Select the row below row k with the best pivot.
		maxRow := k
		for i := k; i < 2; i++ {
			if math.Abs(m[i*2+k]) > math.Abs(m[maxRow*2+k]) {
				maxRow = i
			}
		}

		// Swap selected row to current row.
		for i := 0; i < 2; i++ {
			m[k*2+i], m[maxRow*2+i] = m[maxRow*2+i], m[k*2+i]
			a[k*2+i], a[maxRow*2+i] = a[maxRow*2+i], a[k*2+i]
		}

		// Now the pivot element is at m[k, k].
		pivot := m[k*2+k]
		for r := k + 1; r < 2; r++ {
			scale := m[r*2+k] / pivot
			for c := k + 1; c < 2; c++ {
				m[r*2+c] -= m[k*2+c] * scale
			}
			for c := 0; c < 2; c++ {
				a[r*2+c] -= a[k*2+c] * scale
			}
			m[r*2+k] = 0.0
		}
	}

}

func backsubInplace(m, a *T) {
	for k := 2 - 1; k > 0; k-- {
		// m[k,k] is the pivot

		// Nullify all entries above the pivot element.
		for r := 0; r < k; r++ {
			scale := m[r*2+k] / m[k*2+k]

			m[r*2+k] = 0
			for c := k + 1; c < 2; c++ {
				m[r*2+c] -= m[k*2+c] * scale
			}

			// Mirror the action in the augmented matrix.
			for c := 0; c < 2; c++ {
				a[r*2+c] -= a[k*2+c] * scale
			}
		}
	}

	// Now we simply need to divide each row by its pivot.
	for k := 0; k < 2; k++ {
		for c := k + 1; c < 2; c++ {
			m[k*2+c] /= m[k*2+k]
		}
		for c := 0; c < 2; c++ {
			a[k*2+c] /= m[k*2+k]
		}
		m[k*2+k] = 1
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
