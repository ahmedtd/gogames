package viewport2

import (
	"shellworld/lib/vmath/mat22"
	"shellworld/lib/vmath/mat33"
	"shellworld/lib/vmath/vec2"
)

type Viewport struct {
	Center             vec2.T
	Orientation        mat22.T
	PixelsPerWorldUnit float64
	ScreenWidthPixels  int
	ScreenHeightPixels int
}

func (v *Viewport) WorldToScreenMatrix() mat33.T {
	worldToViewport := mat33.Inverse(mat33.Transform2(1.0, v.Orientation, v.Center))
	worldToViewport = mat33.MulMM(mat33.Scale(v.PixelsPerWorldUnit), worldToViewport)

	vpToScreen := mat33.Scale2(1.0, -1.0)
	vpToScreen = mat33.MulMM(mat33.Translate2(vec2.T{float64(v.ScreenWidthPixels) / 2.0, float64(v.ScreenHeightPixels) / 2.0}), vpToScreen)

	worldToScreen := mat33.MulMM(vpToScreen, worldToViewport)
	return worldToScreen
}

func (v *Viewport) WorldPositionToScreenPosition(worldPos vec2.T) vec2.T {
	return mat33.MulMV2(v.WorldToScreenMatrix(), worldPos)
}

func (v *Viewport) ScreenPositionToWorldPosition(screenPos vec2.T) vec2.T {
	screenToWorld := mat33.Inverse(v.WorldToScreenMatrix())
	worldPos := mat33.MulMV2(screenToWorld, screenPos)
	return worldPos
}
