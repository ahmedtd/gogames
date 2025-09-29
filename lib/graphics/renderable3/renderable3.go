package renderable3

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec2"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
)

type UncompressedImage struct {
	Samples       []float32
	Width, Height int
}

type Tri struct {
	VertexIndices  [3]int
	TextureIndices [3]int
	NormalIndices  [3]int
}

type T struct {
	Vertices      []fvec3.T
	TextureCoords []fvec2.T
	Normals       []fvec3.T

	Tris []Tri

	Diffuse *UncompressedImage
}
