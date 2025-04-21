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
	t1 := ebiten.GeoM{}
	t1.SetElement(0, 0, 1)
	t1.SetElement(0, 1, 0)
	t1.SetElement(0, 2, -float64(entry.Width)/2)
	t1.SetElement(1, 0, 0)
	t1.SetElement(1, 1, -1)
	t1.SetElement(1, 2, float64(entry.Height)/2)

	placeInWorld := ebiten.GeoM{}
	placeInWorld.SetElement(0, 0, modelToWorld[0*3+0])
	placeInWorld.SetElement(0, 1, modelToWorld[0*3+1])
	placeInWorld.SetElement(0, 2, modelToWorld[0*3+2])
	placeInWorld.SetElement(1, 0, modelToWorld[1*3+0])
	placeInWorld.SetElement(1, 1, modelToWorld[1*3+1])
	placeInWorld.SetElement(1, 2, modelToWorld[1*3+2])

	ebWorldToScreen := ebiten.GeoM{}
	ebWorldToScreen.SetElement(0, 0, worldToScreen[0*3+0])
	ebWorldToScreen.SetElement(0, 1, worldToScreen[0*3+1])
	ebWorldToScreen.SetElement(0, 2, worldToScreen[0*3+2])
	ebWorldToScreen.SetElement(1, 0, worldToScreen[1*3+0])
	ebWorldToScreen.SetElement(1, 1, worldToScreen[1*3+1])
	ebWorldToScreen.SetElement(1, 2, worldToScreen[1*3+2])

	dio := &ebiten.DrawImageOptions{}
	dio.GeoM.Concat(t1)
	dio.GeoM.Concat(placeInWorld)
	dio.GeoM.Concat(ebWorldToScreen)

	screen.DrawImage(t.Image.SubImage(image.Rect(entry.X, entry.Y, entry.X+entry.Width, entry.Y+entry.Height)).(*ebiten.Image), dio)
}
