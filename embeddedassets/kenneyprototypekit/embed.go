package kenneyprototypekit

import (
	"bytes"
	"embed"
	"fmt"
	"image/png"
	"io/fs"
	"strings"

	"github.com/ahmedtd/gogames/lib/gltfloader"
	"github.com/ahmedtd/gogames/lib/graphics/renderable3"
)

//go:embed *.glb
var GLBFS embed.FS

//go:embed Textures/colormap.png
var TextureBytes []byte

// Names returns the names of all models in the kit, suitable for passing to
// Load.  (For example, "crate" or "wall-doorway".)
func Names() []string {
	entries, err := fs.ReadDir(GLBFS, ".")
	if err != nil {
		panic(fmt.Sprintf("while reading embedded GLB directory: %v", err))
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, strings.TrimSuffix(e.Name(), ".glb"))
	}
	return names
}

// Load loads the named model mesh and its diffuse texture.  The name is the
// GLB filename without the extension; see Names.
//
// The GLB is Y-up, and is rotated to be Z-up.
func Load(name string) (*renderable3.T, error) {
	glbBytes, err := GLBFS.ReadFile(name + ".glb")
	if err != nil {
		return nil, fmt.Errorf("while reading GLB for %q: %w", name, err)
	}

	mesh, err := gltfloader.ParseGLB(glbBytes, true)
	if err != nil {
		return nil, fmt.Errorf("while parsing GLB for %q: %w", name, err)
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
		// glTF shares one index across all of a vertex's attributes.
		renderable.Tris[i].VertexIndices = mesh.Tris[i]
		renderable.Tris[i].TextureIndices = mesh.Tris[i]
		renderable.Tris[i].NormalIndices = mesh.Tris[i]
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
