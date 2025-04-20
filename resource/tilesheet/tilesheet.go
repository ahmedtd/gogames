package tilesheet

import (
	"image"
	"shellworld/vmath/mat33"
	"shellworld/vmath/vec2"

	"github.com/hajimehoshi/ebiten/v2"
)

type T struct {
	Image *ebiten.Image

	NumTilesX, NumTilesY     int
	TileSizeX, TileSizeY     int
	TileMarginX, TileMarginY int

	// Assume the whole tilesheet shares a common scale
	UnitsPerPixel float64
}

func (t *T) RenderAtWorldPosition(screen *ebiten.Image, tile int, worldToScreen mat33.T, worldCoordinates, worldFacing vec2.T) {
	t1 := ebiten.GeoM{}
	t1.Translate(float64(t.TileSizeX)/2, -float64(t.TileSizeY)/2)
	t1.SetElement(0, 0, 1)
	t1.SetElement(1, 0, 0)
	t1.SetElement(0, 1, 0)
	t1.SetElement(1, 1, -1)

	placeInWorld := ebiten.GeoM{}
	placeInWorld.SetElement(0, 0, worldFacing[0]*t.UnitsPerPixel)
	placeInWorld.SetElement(1, 0, worldFacing[1]*t.UnitsPerPixel)
	placeInWorld.SetElement(0, 1, -worldFacing[1]*t.UnitsPerPixel) // left vector
	placeInWorld.SetElement(1, 1, worldFacing[0]*t.UnitsPerPixel)  // left vector
	placeInWorld.Translate(worldCoordinates[0], worldCoordinates[1])

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

	tileIndexX := tile % t.NumTilesX
	tileIndexY := tile / t.NumTilesX

	sx := tileIndexX*(t.TileMarginX+t.TileSizeX) + t.TileMarginX
	sy := tileIndexY*(t.TileMarginY+t.TileSizeY) + t.TileMarginY

	screen.DrawImage(t.Image.SubImage(image.Rect(sx, sy, sx+t.TileSizeX, sy+t.TileSizeY)).(*ebiten.Image), dio)
}
