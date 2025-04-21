package spritesheet

import (
	"image"
	"shellworld/lib/vmath/mat33"

	"github.com/hajimehoshi/ebiten/v2"
)

type T struct {
	Image *ebiten.Image
}

type AtlasEntry struct {
	X, Y, Width, Height int
}

func (t *T) RenderAtWorldPosition(screen *ebiten.Image, entry *AtlasEntry, worldToScreen, modelToWorld mat33.T) {
	centerSprite := mat33.T{
		1, 0, -float64(entry.Width) / 2,
		0, -1, float64(entry.Height) / 2,
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

	screen.DrawImage(t.Image.SubImage(image.Rect(entry.X, entry.Y, entry.X+entry.Width, entry.Y+entry.Height)).(*ebiten.Image), dio)
}
