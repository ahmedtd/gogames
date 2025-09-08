package kenneycastlekit

import (
	_ "embed"
	"fmt"

	"github.com/ahmedtd/gogames/lib/objloader"
)

//go:embed siege-ballista.obj
var OBJBytes []byte

//go:embed Textures/colormap.png
var TextureBytes []byte

func LoadSiegeBallista() (*objloader.Mesh, error) {
	mesh, err := objloader.Parse(OBJBytes)
	if err != nil {
		return nil, fmt.Errorf("while parsing OBJ: %w", err)
	}

	return mesh, nil
}
