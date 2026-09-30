package fmat33

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec2"

	"github.com/barnex/fmath"
)

type T [9]float32

func Identity() T {
	return T{1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0}
}

func Rotate2(angle float32) T {
	return T{
		fmath.Cos(angle), fmath.Sin(angle), 0.0,
		-fmath.Sin(angle), fmath.Cos(angle), 0.0,
		0.0, 0.0, 1.0,
	}
}

func Scale(s float32) T {
	return T{
		s, 0.0, 0.0,
		0.0, s, 0.0,
		0.0, 0.0, 1.0,
	}
}

func Scale2(sx, sy float32) T {
	return T{
		sx, 0.0, 0.0,
		0.0, sy, 0.0,
		0.0, 0.0, 1.0,
	}
}

func Translate2(t fvec2.T) T {
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

func MulMV2(a T, b fvec2.T) fvec2.T {
	return fvec2.T{
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
			if fmath.Abs(m[i*3+k]) > fmath.Abs(m[maxRow*3+k]) {
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
	a := T{1, 0, 0, 0, 1, 0, 0, 0, 1}
	SolveInplace(&m, &a)
	return a
}

func Orthonormalize(m T) T {
	u1x, u1y, u1z := m[0], m[3], m[6]
	u1len := fmath.Sqrt(u1x*u1x + u1y*u1y + u1z*u1z)
	u1x /= u1len
	u1y /= u1len
	u1z /= u1len

	u2x, u2y, u2z := m[1], m[4], m[7]
	u2dotu1 := u2x*u1x + u2y*u1y + u2z*u1z
	u2x -= u2dotu1 * u1x
	u2y -= u2dotu1 * u1y
	u2z -= u2dotu1 * u1z
	u2len := fmath.Sqrt(u2x*u2x + u2y*u2y + u2z*u2z)
	u2x /= u2len
	u2y /= u2len
	u2z /= u2len

	u3x, u3y, u3z := m[2], m[5], m[8]
	u3dotu1 := u3x*u1x + u3y*u1y + u3z*u1z
	u3x -= u3dotu1 * u1x
	u3y -= u3dotu1 * u1y
	u3z -= u3dotu1 * u1z
	u3dotu2 := u3x*u2x + u3y*u2y + u3z*u2z
	u3x -= u3dotu2 * u2x
	u3y -= u3dotu2 * u2y
	u3z -= u3dotu2 * u2z
	u3len := fmath.Sqrt(u3x*u3x + u3y*u3y + u3z*u3z)
	u3x /= u3len
	u3y /= u3len
	u3z /= u3len

	return T{
		u1x, u2x, u3x,
		u1y, u2y, u3y,
		u1z, u2z, u3z,
	}
}
