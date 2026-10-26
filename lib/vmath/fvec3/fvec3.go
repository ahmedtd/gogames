package fvec3

import (
	"encoding/binary"
	"math"
	"math/rand/v2"

	"github.com/barnex/fmath"
)

type T [4]float32

func (v T) Norm() float32 {
	return fmath.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
}

func Normalize(v T) T {
	l := v.Norm()
	return T{
		v[0] / l,
		v[1] / l,
		v[2] / l,
	}
}

func AddVV(a, b T) T {
	return T{
		a[0] + b[0],
		a[1] + b[1],
		a[2] + b[2],
	}
}

func SubVV(a, b T) T {
	return T{
		a[0] - b[0],
		a[1] - b[1],
		a[2] - b[2],
	}
}

func MulVS(a T, b float32) T {
	return T{
		a[0] * b,
		a[1] * b,
		a[2] * b,
	}
}

func DivVS(a T, b float32) T {
	return T{
		a[0] / b,
		a[1] / b,
		a[2] / b,
	}
}

func IProd(a, b T) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

func CProd(a, b T) T {
	return T{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

// Reject returns the component of b that is orthogonal to A.
func Reject(a, b T) T {
	return SubVV(b, MulVS(Normalize(a), IProd(a, b)/a.Norm()))
}

func Reflect(a, n T) T {
	return SubVV(a, MulVS(n, 2*IProd(a, n)))
}

func UniformSphereDistribution(rng *rand.Rand, radius float32) T {
	result := T{}
	for {
		result[0] = 2 * radius * (rng.Float32() - 0.5)
		result[1] = 2 * radius * (rng.Float32() - 0.5)
		result[2] = 2 * radius * (rng.Float32() - 0.5)
		normSquared := result[0]*result[0] + result[1]*result[1] + result[2]*result[2]
		if normSquared <= radius*radius && normSquared != 0.0 {
			break
		}
	}
	return result
}

func UniformUnitDistribution(rng *rand.Rand, radius float32) T {
	result := T{}
	for {
		result[0] = 2 * radius * (rng.Float32() - 0.5)
		result[1] = 2 * radius * (rng.Float32() - 0.5)
		result[2] = 2 * radius * (rng.Float32() - 0.5)
		normSquared := result[0]*result[0] + result[1]*result[1] + result[2]*result[2]
		if normSquared <= radius*radius && normSquared != 0.0 {
			break
		}
	}
	return Normalize(result)
}

func HemisphereUnitVec3Distribution(normal T, rng *rand.Rand) T {
	candidate := UniformUnitDistribution(rng, 1)
	if IProd(candidate, normal) < 0.0 {
		candidate[0] = -candidate[0]
		candidate[1] = -candidate[1]
		candidate[2] = -candidate[2]
	}
	return candidate
}

func CosineUnitVec3Distribution(normal T, rng *rand.Rand) T {
	for {
		candidate := UniformUnitDistribution(rng, 1)
		cosine := IProd(normal, candidate)

		if cosine < 0.0 {
			cosine = -cosine
			candidate[0] = -candidate[0]
			candidate[1] = -candidate[1]
			candidate[2] = -candidate[2]
		}

		// TODO: Shouldn't it be fine to use a rejection sample from [0.0, 1.0)?
		// We know cosine is always nonnegative.
		rejectionSample := rng.Float32()*2.0 - 1.0
		if rejectionSample >= cosine {
			return candidate
		}
	}
	// Dead code.
	return T{0, 0, 0}
}

func GaussianUnitVec3Distribution(normal T, mid float32, rng *rand.Rand) T {
	// By cutting the case variance == 0 from the rejection testing, we
	// ensure that the loop below will terminate, since the result of exp
	// will never be NaN.

	// The loop could, however, take a very long time indeed for small
	// variance values, since the pdf evaluates to zero for larger and
	// larger fractions of the rejection test interval as v gets larger.

	// In general, for double precision arithmetic, truly problematic values
	// will start as v gets smaller than ~(1/1400).

	for {
		candidate := UniformUnitDistribution(rng, 1)
		cosine := IProd(normal, candidate)
		if cosine < 0.0 {
			cosine = -cosine
			candidate[0] = -candidate[0]
			candidate[1] = -candidate[1]
			candidate[2] = -candidate[2]
		}

		// PDF is a triangle with peak at `cosine == mid`
		pdfVal := float32(0.0)
		if cosine < mid {
			pdfVal = cosine / mid
		} else {
			pdfVal = -(cosine-mid)/(1-mid) + 1
		}

		rejectionSample := rng.Float32()
		if rejectionSample >= pdfVal {
			return candidate
		}
	}

	// Dead code.
	return T{0, 0, 0}
}

func AppendWGPUBytes(m *T, b []byte) []byte {
	// vec3<f32> AlignOf=16 SizeOf=12 (WGPU Spec)

	// We need to make sure the point we're about to write to is 16-byte aligned
	// (counting from the beginning of b).
	alignTo := alignUp(len(b), 16)
	alignmentPadding := alignTo - len(b)
	b = append(b, make([]byte, alignmentPadding)...)

	b = binary.LittleEndian.AppendUint32(b, math.Float32bits(m[0]))
	b = binary.LittleEndian.AppendUint32(b, math.Float32bits(m[1]))
	b = binary.LittleEndian.AppendUint32(b, math.Float32bits(m[2]))
	return b
}

func alignUp(n, a int) int {
	return (n + a - 1) / a * a
}
