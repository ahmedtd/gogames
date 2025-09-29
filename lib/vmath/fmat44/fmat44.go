package fmat44

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
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

func MulMV(a T, b fvec4.T) fvec4.T {
	return fvec4.T{
		a[0]*b[0] + a[1]*b[1] + a[2]*b[2] + a[3]*b[3],
		a[4]*b[0] + a[5]*b[1] + a[6]*b[2] + a[7]*b[3],
		a[8]*b[0] + a[9]*b[1] + a[10]*b[2] + a[11]*b[3],
		a[12]*b[0] + a[13]*b[1] + a[14]*b[2] + a[15]*b[3],
	}
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
