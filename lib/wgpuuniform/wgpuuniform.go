package wgpuuniform

import (
	"encoding/binary"
	"math"

	"github.com/ahmedtd/gogames/lib/vmath/fmat33"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
)

func AppendFloat32(f float32, b []byte) []byte {
	// align=4 size=4

	// Pad to alignment.
	b = PadTo(4, b)

	b = binary.LittleEndian.AppendUint32(b, math.Float32bits(f))

	return b
}

func AppendFVec3(m *fvec3.T, b []byte) []byte {
	// vec3<f32> AlignOf=16 SizeOf=12 (WGPU Spec)

	// We need to make sure the point we're about to write to is 16-byte aligned
	// (counting from the beginning of b).
	b = PadTo(16, b)

	b = AppendFloat32(m[0], b)
	b = AppendFloat32(m[1], b)
	b = AppendFloat32(m[2], b)
	return b
}

func AppendFMat33(m *fmat33.T, b []byte) []byte {
	// mat3x3<f32> AlignOf=16 SizeOf=48 (WGPU Spec)

	// We need to make sure the point we're about to write to is 16-byte aligned
	// (counting from the beginning of b).
	b = PadTo(16, b)

	b = AppendFloat32(m[0], b)
	b = AppendFloat32(m[3], b)
	b = AppendFloat32(m[6], b)
	b = append(b, 0, 0, 0, 0)

	b = AppendFloat32(m[1], b)
	b = AppendFloat32(m[4], b)
	b = AppendFloat32(m[7], b)
	b = append(b, 0, 0, 0, 0)

	b = AppendFloat32(m[2], b)
	b = AppendFloat32(m[5], b)
	b = AppendFloat32(m[8], b)
	b = append(b, 0, 0, 0, 0)

	return b
}

func PadTo(alignment int, b []byte) []byte {
	alignTo := alignUp(len(b), alignment)
	alignmentPadding := alignTo - len(b)
	for i := 0; i < alignmentPadding; i++ {
		b = append(b, 0)
	}
	return b
}

func alignUp(n, a int) int {
	return (n + a - 1) / a * a
}
