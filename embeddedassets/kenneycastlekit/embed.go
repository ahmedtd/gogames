package kenneycastlekit

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"

	"github.com/ahmedtd/gogames/lib/graphics/renderable3"
	"github.com/ahmedtd/gogames/lib/objloader"
)

//go:embed siege-ballista.obj
var OBJBytes []byte

//go:embed Textures/colormap.png
var TextureBytes []byte

// LoadSiegeBallista loads the siege ballista mesh and its diffuse texture.
//
// The OBJ is Y-up, and is converted to Z-up while parsing.  That conversion
// leaves the model upside down in a right-handed Z-up world, so flipZ rotates
// it 180 degrees about the X axis to stand it upright.  (Y is negated along
// with Z so that the model isn't mirrored.)
func LoadSiegeBallista(flipZ bool) (*renderable3.T, error) {
	mesh, err := objloader.Parse(OBJBytes, true)
	if err != nil {
		return nil, fmt.Errorf("while parsing OBJ: %w", err)
	}

	if flipZ {
		for i := range mesh.Vertices {
			mesh.Vertices[i][1] = -mesh.Vertices[i][1]
			mesh.Vertices[i][2] = -mesh.Vertices[i][2]
		}
		for i := range mesh.Normals {
			mesh.Normals[i][1] = -mesh.Normals[i][1]
			mesh.Normals[i][2] = -mesh.Normals[i][2]
		}
	}

	diffuseImage, err := png.Decode(bytes.NewBuffer(TextureBytes))
	if err != nil {
		return nil, fmt.Errorf("while parsing diffuse PNG: %w", err)
	}

	renderable := &renderable3.T{
		Vertices:      mesh.Vertices,
		TextureCoords: mesh.TextureCoords,
		Normals:       mesh.Normals,
	}
	renderable.Tris = make([]renderable3.Tri, len(mesh.Tris))
	for i := range mesh.Tris {
		renderable.Tris[i].VertexIndices = mesh.Tris[i].VertexIndices
		renderable.Tris[i].TextureIndices = mesh.Tris[i].TextureIndices
		renderable.Tris[i].NormalIndices = mesh.Tris[i].NormalIndices
	}

	renderable.Diffuse = &renderable3.UncompressedImage{
		Width:  diffuseImage.Bounds().Max.X,
		Height: diffuseImage.Bounds().Max.Y,
	}
	renderable.Diffuse.Samples = make([]float32, renderable.Diffuse.Height*renderable.Diffuse.Width*4)
	for y := 0; y < renderable.Diffuse.Height; y++ {
		for x := 0; x < renderable.Diffuse.Width; x++ {
			r, g, b, a := diffuseImage.At(x, y).RGBA()
			renderable.Diffuse.Samples[y*renderable.Diffuse.Width*4+x*4+0] = float32(r) / 65535
			renderable.Diffuse.Samples[y*renderable.Diffuse.Width*4+x*4+1] = float32(g) / 65535
			renderable.Diffuse.Samples[y*renderable.Diffuse.Width*4+x*4+2] = float32(b) / 65535
			renderable.Diffuse.Samples[y*renderable.Diffuse.Width*4+x*4+3] = float32(a) / 65535
		}
	}

	return renderable, nil
}
