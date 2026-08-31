package af32

import (
	"fmt"
	"unsafe"
)

type T struct {
	V     []float32
	Shape []int
}

func Make(shape ...int) *T {
	for _, s := range shape {
		if s <= 0 {
			panic(fmt.Sprintf("invalid shape: %v", shape))
		}
	}
	size := 1
	for _, s := range shape {
		size *= s
	}

	return &T{
		V:     make([]float32, size),
		Shape: shape,
	}
}

func MakeScalar(scalar float32) *T {
	return &T{
		V:     []float32{scalar},
		Shape: []int{1},
	}
}

func Copy(in *T) *T {
	shapeCopy := make([]int, len(in.Shape))
	copy(shapeCopy, in.Shape)
	vCopy := make([]float32, len(in.V))
	copy(vCopy, in.V)
	return &T{
		V:     vCopy,
		Shape: shapeCopy,
	}
}

func Transpose(in *T, out *T) {
	if len(in.Shape) != 2 {
		panic("cannot transpose if len(shape) != 2")
	}
	if len(in.V) != len(out.V) {
		panic("output storage is not correctly sized to store the transpose of the input")
	}
	out.Shape = []int{in.Shape[1], in.Shape[0]}

	for i := 0; i < in.Shape[0]; i++ {
		for j := 0; j < in.Shape[1]; j++ {
			out.Set2(j, i, in.At2(i, j))
		}
	}
}

// Reshape reshapes the input tensor.  The overall number of elements must
// be the same.  The returned tensor shares storage with the input tensor (no
// data is copied).
func Reshape(a *T, shape ...int) *T {
	newSize := 1
	for _, s := range shape {
		if s <= 0 {
			panic(fmt.Sprintf("invalid shape: %v", shape))
		}
		newSize *= s
	}

	if newSize != len(a.V) {
		panic("invalid reshape")
	}

	return &T{
		V:     a.V,
		Shape: shape,
	}
}

func (a *T) At1(idx int) float32 {
	// pBase := unsafe.Pointer(unsafe.SliceData(a.V))
	// pElt := (*float32)(unsafe.Pointer(uintptr(pBase) + uintptr(idx*4)))
	// return *pElt
	return a.V[idx]
}

func (a *T) At2(idx0, idx1 int) float32 {
	if len(a.Shape) != 2 {
		panic("At2() invalid for len(shape) != 2")
	}
	return a.V[idx0*a.Shape[1]+idx1]
}

func (a *T) At3(idx0, idx1, idx2 int) float32 {
	if len(a.Shape) != 3 {
		panic("At3() invalid for len(shape) != 3")
	}
	return a.V[idx0*a.Shape[1]*a.Shape[2]+idx1*a.Shape[2]+idx2]
}

func (a *T) CheckedAt3(idx0, idx1, idx2 int) float32 {
	return a.V[idx0*a.Shape[1]*a.Shape[2]+idx1*a.Shape[2]+idx2]
}

func (a *T) Set1(idx int, v float32) {
	pBase := unsafe.Pointer(unsafe.SliceData(a.V))
	pElt := (*float32)(unsafe.Pointer(uintptr(pBase) + uintptr(idx*4)))
	*pElt = v
	// a.V[idx] = v
}

func (a *T) Set2(idx0, idx1 int, v float32) {
	if len(a.Shape) != 2 {
		panic("Set2() invalid for len(shape) != 2")
	}
	a.V[idx0*a.Shape[1]+idx1] = v
}

func (a *T) Set3(idx0, idx1, idx2 int, v float32) {
	if len(a.Shape) != 3 {
		panic("Set3 only valid for rank-3 AF32")
	}
	a.V[idx0*a.Shape[1]*a.Shape[2]+idx1*a.Shape[2]+idx2] = v
}
