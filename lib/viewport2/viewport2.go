package viewport2

import (
	"github.com/ahmedtd/gogames/lib/vmath/aabox2"
	"github.com/ahmedtd/gogames/lib/vmath/mat22"
	"github.com/ahmedtd/gogames/lib/vmath/mat33"
	"github.com/ahmedtd/gogames/lib/vmath/vec2"
)

type Viewport struct {
	Center             vec2.T
	Orientation        mat22.T
	PixelsPerWorldUnit float64

	ScreenWidthPixels  int
	ScreenHeightPixels int

	WorldToScreenMatrix mat33.T
	ScreenToWorldMatrix mat33.T
	WorldAABox          aabox2.T
}

func (v *Viewport) ComputeForFrame(screenWidthPixels, screenHeightPixels int) {
	v.ScreenWidthPixels = screenWidthPixels
	v.ScreenHeightPixels = screenHeightPixels

	worldToViewport := mat33.Inverse(mat33.Transform2(1.0, v.Orientation, v.Center))
	worldToViewport = mat33.MulMM(mat33.Scale(v.PixelsPerWorldUnit), worldToViewport)

	vpToScreen := mat33.Scale2(1.0, -1.0)
	vpToScreen = mat33.MulMM(mat33.Translate2(vec2.T{float64(v.ScreenWidthPixels) / 2.0, float64(v.ScreenHeightPixels) / 2.0}), vpToScreen)

	worldToScreen := mat33.MulMM(vpToScreen, worldToViewport)

	v.WorldToScreenMatrix = worldToScreen
	v.ScreenToWorldMatrix = mat33.Inverse(worldToScreen)

	v.WorldAABox = aabox2.OfPoints(
		vec2.T{0, 0},
		vec2.T{0, float64(screenHeightPixels)},
		vec2.T{float64(screenWidthPixels), 0},
		vec2.T{float64(screenWidthPixels), float64(screenHeightPixels)},
	)
}

func (v *Viewport) WorldPositionToScreenPosition(worldPos vec2.T) vec2.T {
	return mat33.MulMV2(v.WorldToScreenMatrix, worldPos)
}

func (v *Viewport) ScreenPositionToWorldPosition(screenPos vec2.T) vec2.T {
	return mat33.MulMV2(v.ScreenToWorldMatrix, screenPos)
}
