package fmat44

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
	"github.com/barnex/fmath"
)

type T [16]float32

func Identity() T {
	return T{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// Viewport produces a 4x4 affine transformation matrix that maps from clip
// space to screen coords.
func Viewport(w, h float32) T {
	return T{
		w / 2, 0, 0, w / 2,
		0, h / 2, 0, h / 2,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

func Perspective(f float32) T {
	return T{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, -1 / f, 1,
	}
}

func LookAt(camPos, targetPos, globalUp fvec3.T) T {
	eye := fvec3.Normalize(fvec3.SubVV(targetPos, camPos))
	left := fvec3.Normalize(fvec3.CProd(globalUp, eye))
	camUp := fvec3.Normalize(fvec3.CProd(eye, left))

	changeOfBasis := T{
		left[0], left[1], left[2], 0,
		camUp[0], camUp[1], camUp[2], 0,
		eye[0], eye[1], eye[2], 0,
		0, 0, 0, 1,
	}

	translation := T{
		1, 0, 0, -camPos[0],
		0, 1, 0, -camPos[1],
		0, 0, 1, -camPos[2],
		0, 0, 0, 1,
	}

	return MulMM(changeOfBasis, translation)
}

func MulMM(a, b T) T {
	result := T{}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				result[i*4+j] += a[i*4+k] * b[k*4+j]
			}
		}
	}
	return result
}

func MulMV3(a T, b fvec3.T) fvec3.T {
	return fvec3.T{
		a[0]*b[0] + a[1]*b[1] + a[2]*b[2] + a[3]*1,
		a[4]*b[0] + a[5]*b[1] + a[6]*b[2] + a[7]*1,
		a[8]*b[0] + a[9]*b[1] + a[10]*b[2] + a[11]*1,
	}
}

func rowEchelonInplace(m, a *T) {
	for k := 0; k < 4; k++ {
		// Select the row below row k with the best pivot.
		maxRow := k
		for i := k; i < 4; i++ {
			if fmath.Abs(m[i*4+k]) > fmath.Abs(m[maxRow*4+k]) {
				maxRow = i
			}
		}

		// Swap selected row to current row.
		for i := 0; i < 4; i++ {
			m[k*4+i], m[maxRow*4+i] = m[maxRow*4+i], m[k*4+i]
			a[k*4+i], a[maxRow*4+i] = a[maxRow*4+i], a[k*4+i]
		}

		// Now the pivot element is at m[k, k].
		pivot := m[k*4+k]
		for r := k + 1; r < 4; r++ {
			scale := m[r*4+k] / pivot
			for c := k + 1; c < 4; c++ {
				m[r*4+c] -= m[k*4+c] * scale
			}
			for c := 0; c < 4; c++ {
				a[r*4+c] -= a[k*4+c] * scale
			}
			m[r*4+k] = 0.0
		}
	}
}

func backsubInplace(m, a *T) {
	for k := 4 - 1; k > 0; k-- {
		// m[k,k] is the pivot

		// Nullify all entries above the pivot element.
		for r := 0; r < k; r++ {
			scale := m[r*4+k] / m[k*4+k]

			m[r*4+k] = 0
			for c := k + 1; c < 4; c++ {
				m[r*4+c] -= m[k*4+c] * scale
			}

			// Mirror the action in the augmented matrix.
			for c := 0; c < 4; c++ {
				a[r*4+c] -= a[k*4+c] * scale
			}
		}
	}

	// Now we simply need to divide each row by its pivot.
	for k := 0; k < 4; k++ {
		for c := k + 1; c < 4; c++ {
			m[k*4+c] /= m[k*4+k]
		}
		for c := 0; c < 4; c++ {
			a[k*4+c] /= m[k*4+k]
		}
		m[k*4+k] = 1
	}
}

func SolveInplace(m, a *T) {
	rowEchelonInplace(m, a)
	backsubInplace(m, a)
}

func Inverse(m T) T {
	a := T{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
	SolveInplace(&m, &a)
	return a
}

// Orthonormalize re-orthonormalizes the upper-left 3x3 rotation block of m
// via Gram-Schmidt, leaving the last row and column set to the identity
// extension (0, 0, 0, 1).
func Orthonormalize(m T) T {
	u1x, u1y, u1z := m[0], m[4], m[8]
	u1len := fmath.Sqrt(u1x*u1x + u1y*u1y + u1z*u1z)
	u1x /= u1len
	u1y /= u1len
	u1z /= u1len

	u2x, u2y, u2z := m[1], m[5], m[9]
	u2dotu1 := u2x*u1x + u2y*u1y + u2z*u1z
	u2x -= u2dotu1 * u1x
	u2y -= u2dotu1 * u1y
	u2z -= u2dotu1 * u1z
	u2len := fmath.Sqrt(u2x*u2x + u2y*u2y + u2z*u2z)
	u2x /= u2len
	u2y /= u2len
	u2z /= u2len

	u3x, u3y, u3z := m[2], m[6], m[10]
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
		u1x, u2x, u3x, 0,
		u1y, u2y, u3y, 0,
		u1z, u2z, u3z, 0,
		0, 0, 0, 1,
	}
}
