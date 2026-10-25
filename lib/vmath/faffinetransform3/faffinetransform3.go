package faffinetransform3

import (
	"github.com/ahmedtd/gogames/lib/vmath/fmat33"
	"github.com/ahmedtd/gogames/lib/vmath/fmat44"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"

	"github.com/barnex/fmath"
)

type T struct {
	Linear fmat33.T
	Offset fvec3.T
}

func Identity() T {
	return T{
		Linear: fmat33.T{1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0},
		Offset: fvec3.T{0.0, 0.0, 0.0},
	}
}

func Scale(s float32) T {
	return T{
		Linear: fmat33.T{s, 0.0, 0.0, 0.0, s, 0.0, 0.0, 0.0, s},
		Offset: fvec3.T{0.0, 0.0, 0.0},
	}
}

func Translate(x fvec3.T) T {
	result := Identity()
	result.Offset = x
	return result
}

func AxisAngle(u fvec3.T, angle float32) T {
	c := fmath.Cos(angle)
	s := fmath.Sin(angle)

	ux := u[0]
	uy := u[1]
	uz := u[2]

	return T{
		Linear: fmat33.T{
			ux*ux*(1-c) + c, ux*uy*(1-c) - uz*s, ux*uz*(1-c) + uy*s, //
			ux*uy*(1-c) + uz*s, uy*uy*(1-c) + c, uy*uz*(1-c) - ux*s, //
			ux*uz*(1-c) - uy*s, uy*uz*(1-c) + ux*s, uz*uz*(1-c) + c,
		},

		Offset: fvec3.T{0.0, 0.0, 0.0},
	}

}

func Compose(a, b T) T {
	return T{
		Linear: fmat33.MulMM(a.Linear, b.Linear),
		Offset: fvec3.AddVV(a.Offset, fmat33.MulMV(a.Linear, b.Offset)),
	}
}

func (t T) Invert() T {
	mat := fmat44.T{
		t.Linear[0], t.Linear[1], t.Linear[2], t.Offset[0],
		t.Linear[3], t.Linear[4], t.Linear[5], t.Offset[1],
		t.Linear[6], t.Linear[7], t.Linear[8], t.Offset[2],
		0, 0, 0, 1,
	}

	inv := fmat44.Inverse(mat)

	return T{
		Linear: fmat33.T{
			inv[0], inv[1], inv[2],
			inv[4], inv[5], inv[6],
			inv[8], inv[9], inv[10],
		},
		Offset: fvec3.T{inv[3], inv[7], inv[11]},
	}
}

func (t T) NormalTransformMat() fmat33.T {
	return fmat33.Transpose(fmat33.Inverse(t.Linear))
}

func (t T) As3x4() [12]float32 {
	return [12]float32{
		t.Linear[0], t.Linear[1], t.Linear[2], t.Offset[0],
		t.Linear[3], t.Linear[4], t.Linear[5], t.Offset[1],
		t.Linear[6], t.Linear[7], t.Linear[8], t.Offset[2],
	}
}

func TransformPoint(a T, b fvec3.T) fvec3.T {
	return fvec3.AddVV(fmat33.MulMV(a.Linear, b), a.Offset)
}
