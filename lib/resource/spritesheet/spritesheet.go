package spritesheet

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"path"

	"github.com/ahmedtd/gogames/lib/resource/spritesheet/spritesheetpb"
	"github.com/ahmedtd/gogames/lib/vmath/mat33"
	"github.com/hajimehoshi/ebiten/v2"
	"google.golang.org/protobuf/encoding/prototext"
)

type T struct {
	Proto *spritesheetpb.SpriteSheet
	Image *ebiten.Image
}

func New(sheetProto *spritesheetpb.SpriteSheet, image *ebiten.Image) *T {
	return &T{
		Proto: sheetProto,
		Image: image,
	}
}

func LoadFromFS(fSys fs.ReadFileFS, protoFileName string) (*T, error) {
	// Using path instead of filepath because in WSL, I have to pass
	// GOOS=windows in order for ebiten to work.

	loadDir := path.Dir(protoFileName)

	textProtoBytes, err := fSys.ReadFile(protoFileName)
	if err != nil {
		return nil, fmt.Errorf("while reading textproto file %q: %w", protoFileName, err)
	}

	sheetProto := &spritesheetpb.SpriteSheet{}
	if err := prototext.Unmarshal(textProtoBytes, sheetProto); err != nil {
		return nil, fmt.Errorf("while unmarshaling spritesheet from textproto: %w", err)
	}

	imgPath := path.Join(loadDir, sheetProto.ImagePath)

	imgBytes, err := fSys.ReadFile(imgPath)
	if err != nil {
		return nil, fmt.Errorf("while reading image bytes: %w", err)
	}

	pngImg, err := png.Decode(bytes.NewBuffer(imgBytes))
	if err != nil {
		return nil, fmt.Errorf("while decoding PNG: %w", err)
	}

	ret := &T{
		Proto: sheetProto,
		Image: ebiten.NewImageFromImage(pngImg),
	}

	return ret, nil
}

type AtlasEntry struct {
	X, Y, Width, Height int
	CenterX, CenterY    float32
}

func (t *T) RenderAtWorldPosition(screen *ebiten.Image, entryIndex int, worldToScreen, modelToWorld mat33.T) {
	entry := t.Proto.GetSprites()[entryIndex]

	centerSprite := mat33.T{
		1, 0, -float64(entry.GetCenterX()),
		0, -1, float64(entry.GetCenterY()),
		0, 0, 1,
	}

	finalMat := mat33.MulMM(worldToScreen, mat33.MulMM(modelToWorld, centerSprite))

	dio := &ebiten.DrawImageOptions{}
	dio.GeoM.SetElement(0, 0, finalMat[0*3+0])
	dio.GeoM.SetElement(0, 1, finalMat[0*3+1])
	dio.GeoM.SetElement(0, 2, finalMat[0*3+2])
	dio.GeoM.SetElement(1, 0, finalMat[1*3+0])
	dio.GeoM.SetElement(1, 1, finalMat[1*3+1])
	dio.GeoM.SetElement(1, 2, finalMat[1*3+2])

	screen.DrawImage(t.Image.SubImage(image.Rect(int(entry.GetX()), int(entry.GetY()), int(entry.GetX()+entry.GetWidth()), int(entry.GetY()+entry.GetHeight()))).(*ebiten.Image), dio)
}
