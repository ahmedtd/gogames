package imagestamp

import (
	"github.com/ahmedtd/gogames/lib/vmath/mat33"
	"github.com/ahmedtd/gogames/lib/vmath/vec2"

	"github.com/hajimehoshi/ebiten/v2"
)

type Tint [4]float64

type ImageStamp struct {
	image            *ebiten.Image
	drawImageOptions *ebiten.DrawImageOptions
	// center of the armature in image pixel coordinates.
	center         vec2.T
	metersPerPixel float64
}

func New(image *ebiten.Image, center vec2.T, widthInMeters float64) *ImageStamp {
	return &ImageStamp{
		image:            image,
		drawImageOptions: &ebiten.DrawImageOptions{},
		center:           center,
		metersPerPixel:   widthInMeters / float64(image.Bounds().Dx()),
	}
}

func (s *ImageStamp) RenderOnWorld(screen *ebiten.Image, worldToScreen mat33.T, worldCoordinates, worldFacing vec2.T, tint Tint) {
	t1 := ebiten.GeoM{}
	t1.Translate(s.center[0], -s.center[1])
	t1.SetElement(0, 0, 0)
	t1.SetElement(1, 0, 1)
	t1.SetElement(0, 1, -1)
	t1.SetElement(1, 1, 0)
	// Flip the image, so that y on the image is east in the world.

	t2 := ebiten.GeoM{}
	t2.SetElement(0, 0, worldFacing[0]*s.metersPerPixel)
	t2.SetElement(1, 0, worldFacing[1]*s.metersPerPixel)
	t2.SetElement(0, 1, -worldFacing[1]*s.metersPerPixel) // left vector
	t2.SetElement(1, 1, worldFacing[0]*s.metersPerPixel)  // left vector
	t2.Translate(worldCoordinates[0], worldCoordinates[1])

	ebWorldToScreen := ebiten.GeoM{}
	ebWorldToScreen.SetElement(0, 0, worldToScreen[0*3+0])
	ebWorldToScreen.SetElement(0, 1, worldToScreen[0*3+1])
	ebWorldToScreen.SetElement(0, 2, worldToScreen[0*3+2])
	ebWorldToScreen.SetElement(1, 0, worldToScreen[1*3+0])
	ebWorldToScreen.SetElement(1, 1, worldToScreen[1*3+1])
	ebWorldToScreen.SetElement(1, 2, worldToScreen[1*3+2])

	s.drawImageOptions.GeoM.Reset()
	s.drawImageOptions.GeoM.Concat(t1)
	s.drawImageOptions.GeoM.Concat(t2)
	s.drawImageOptions.GeoM.Concat(ebWorldToScreen)

	s.drawImageOptions.ColorM.Reset()
	s.drawImageOptions.ColorM.Scale(tint[0], tint[1], tint[2], tint[3])

	screen.DrawImage(s.image, s.drawImageOptions)
}
