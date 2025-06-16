package kenneyminicharacters

import (
	"embed"
	_ "embed"
)

//go:embed character-female-a.glb
//go:embed Textures/colormap.png
var content embed.FS

// func LoadCharacterFemaleA() (*gltf.Document, error) {
// 	doc, err := gltf.Open("embeddedassets/kenneyminicharacters/character-female-a.glb")
// 	if err != nil {
// 		return nil, fmt.Errorf("while loading glb: %w", err)
// 	}

// 	doc.Meshes[0].Primitives[0].Targets[gltf.TargetPo]

// 	return doc
// }
