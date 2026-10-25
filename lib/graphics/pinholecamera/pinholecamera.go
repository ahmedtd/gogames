package pinholecamera

import (
	"github.com/ahmedtd/gogames/lib/vmath/fmat33"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
	"github.com/barnex/fmath"
)

type PinholeCamera struct {
	Center          fvec3.T
	ApertureToWorld fmat33.T
	Aperture        fvec3.T
}

// FitAperture sizes the camera's aperture to match the ratio of an rx by ry
// image.
func (c *PinholeCamera) FitAperture(rx, ry int) {
	// Horizontal FOV is 80 degrees, with vertical FOV adjusted to match.
	c.Aperture[0] = 1
	c.Aperture[1] = fmath.Sin(40.0 / 180.0 * 2.0 * fmath.Pi)
	c.Aperture[2] = c.Aperture[1] / float32(rx) * float32(ry)
}

// ApertureRay returns the unit direction, in aperture space, of the camera ray
// through raster position (x, y) of an rx by ry image.  Raster positions are
// in pixels, with x increasing rightwards and y downwards from the top-left
// corner of the image; pixel (px, py) covers [px, px+1) x [py, py+1).
func (c *PinholeCamera) ApertureRay(x, y float32, rx, ry int) fvec3.T {
	return fvec3.Normalize(fvec3.T{
		c.Aperture[0],
		(1.0 - 2.0*x/float32(rx)) * c.Aperture[1],
		(1.0 - 2.0*y/float32(ry)) * c.Aperture[2],
	})
}

func (c *PinholeCamera) Eye() fvec3.T {
	return fvec3.T{
		c.ApertureToWorld[0],
		c.ApertureToWorld[3],
		c.ApertureToWorld[6],
	}
}

func (c *PinholeCamera) Left() fvec3.T {
	return fvec3.T{
		c.ApertureToWorld[1],
		c.ApertureToWorld[4],
		c.ApertureToWorld[7],
	}
}

func (c *PinholeCamera) Up() fvec3.T {
	return fvec3.T{
		c.ApertureToWorld[2],
		c.ApertureToWorld[5],
		c.ApertureToWorld[8],
	}
}

func (c *PinholeCamera) SetEye(newEye fvec3.T) {
	c.setEyeDirect(fvec3.Normalize(newEye))
	c.setUpDirect(fvec3.Normalize(fvec3.Reject(c.Eye(), c.Up())))
	c.setLeftDirect(fvec3.CProd(c.Up(), c.Eye()))
}

func (c *PinholeCamera) SetUp(newUp fvec3.T) {
	c.setUpDirect(fvec3.Normalize(fvec3.Reject(c.Eye(), newUp)))
	c.setLeftDirect(fvec3.CProd(c.Up(), c.Eye()))
}

func (c *PinholeCamera) setEyeDirect(newEye fvec3.T) {
	c.ApertureToWorld[0] = newEye[0]
	c.ApertureToWorld[3] = newEye[1]
	c.ApertureToWorld[6] = newEye[2]
}

func (c *PinholeCamera) setLeftDirect(newLeft fvec3.T) {
	c.ApertureToWorld[1] = newLeft[0]
	c.ApertureToWorld[4] = newLeft[1]
	c.ApertureToWorld[7] = newLeft[2]
}

func (c *PinholeCamera) setUpDirect(newUp fvec3.T) {
	c.ApertureToWorld[2] = newUp[0]
	c.ApertureToWorld[5] = newUp[1]
	c.ApertureToWorld[8] = newUp[2]
}
