package main

// Dependency on ebitengine pulls in glfw, etc.
//
// For now, run with:
//
//    go run ./games/renderino/cmd/renderino

import (
	"bytes"
	"flag"
	"fmt"
	"image/color"
	"log"
	"math"
	"sync"
	"time"

	"shellworld/games/renderino/resources"
	"shellworld/vmath/fmat33"
	"shellworld/vmath/fvec3"

	"github.com/barnex/fmath"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/sync/errgroup"
)

func main() {
	flag.Parse()

	if err := do(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func do() error {
	renderer, err := NewRenderer()
	if err != nil {
		return fmt.Errorf("while creating renderer: %w", err)
	}
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("renderino")
	if err := ebiten.RunGame(renderer); err != nil {
		return fmt.Errorf("while running game: %w", err)
	}

	return nil
}

type Renderer struct {
	SimTime float64

	LatestRenderState *RenderState

	// Draws per second calculation.  Ebiten's is noisy.
	lastDrawStart time.Time

	// If the framerate is too low, UpscaleFactor will be used to drop the
	// amount of work the raytracer has to do.
	curUpscaleFactor        float64
	upscaleTuningIntegrator float64

	renderGroup *errgroup.Group

	TileSize int
	tiles    []RenderTile

	Buf     []byte
	bufLock sync.Mutex

	RGBPowerBuf []RGBPower

	uiFaceSource *text.GoTextFaceSource
	uiNormalFace *text.GoTextFace
}

type RenderTile struct {
	SrcX, SrcY  int
	LimX, LimY  int
	RGBPowerBuf []RGBPower
}

var _ ebiten.Game = &Renderer{}

func NewRenderer() (*Renderer, error) {
	camera := &PinholeCamera{
		Center:          fvec3.T{-10, 0, 0},
		ApertureToWorld: fmat33.T{1, 0, 0, 0, 1, 0, 0, 0, 1},
		Aperture:        fvec3.T{0.02, 0.018, 0.012},
	}
	camera.SetEye(fvec3.T{1.0, 0.0, 0.0})
	camera.SetUp(fvec3.T{0.0, 0.0, 1.0})

	renderGroup := &errgroup.Group{}
	renderGroup.SetLimit(1)

	uiFaceSource, err := text.NewGoTextFaceSource(bytes.NewReader(resources.DepartureMonoOTF))
	if err != nil {
		return nil, fmt.Errorf("while loading Departure Mono font: %w", err)
	}

	r := &Renderer{
		curUpscaleFactor: 1.0,

		renderGroup: renderGroup,

		TileSize: 128,

		uiFaceSource: uiFaceSource,
		uiNormalFace: &text.GoTextFace{
			Source: uiFaceSource,
			Size:   24,
		},

		LatestRenderState: &RenderState{
			Camera: camera,

			Spheres: []Sphere{
				{Center: fvec3.T{10.0, 0.0, 0.0}},
			},
			AmbientLights: []AmbientLight{
				{Power: RGBPower{0.2, 0.2, 0.2}},
			},
			DirectionalLights: []DirectionalLight{
				{
					DirectionToSource: fvec3.T{0, 0, 1},
					Power:             RGBPower{1.0, 1.0, 1.0},
				},
			},
			PointLights: []PointLight{
				{
					Center: fvec3.T{5, 0, 0},
					Power:  RGBPower{1.0, 0.0, 0.0},
				},
			},
		},
	}
	return r, nil
}

func (r *Renderer) Update() error {
	r.SimTime += 1 / 60.0

	r.LatestRenderState.PointLights[0].Center = fvec3.T{
		float32(math.Cos(r.SimTime)),
		float32(math.Sin(r.SimTime)),
		2.0,
	}

	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	drawStart := time.Now()
	drawDelta := drawStart.Sub(r.lastDrawStart)
	if r.lastDrawStart.IsZero() {
		drawDelta = time.Second / 60
	}

	// Compute upscaling factor based on how long it has been since the last
	// draw call.  Adjusting r.UpscaleFactor won't actually take effect until
	// the next call to r.Layout().
	//
	// Note that because ebiten is targeting 60FPS, the error should only go
	// slightly negative.  Without that logic, we would need additional logic
	// because the controller would try to lower the upscaling factor below 1 to
	// do more work per frame.
	drawDeltaError := drawDelta.Seconds() - (1.0 / 60.0)
	r.upscaleTuningIntegrator = 0.9*r.upscaleTuningIntegrator + drawDeltaError
	newUpscaleFactor := 1.0 + 10.0*drawDeltaError + 1.0*r.upscaleTuningIntegrator
	if newUpscaleFactor < 1.0 {
		// Dynamic upscaling is not allowed to try to introduce more work to
		// increase the frame time.
		newUpscaleFactor = 1.0
	}
	if math.Abs(newUpscaleFactor-r.curUpscaleFactor) > 0.1 || newUpscaleFactor == 1.0 {
		// If upscaler control is proposing a real change, or targeting full
		// resolution, apply it.
		r.curUpscaleFactor = newUpscaleFactor
	}

	// log.Printf("drawDelta=%v curUpscaleFactor=%f", drawDelta, r.curUpscaleFactor)

	sx := screen.Bounds().Dx()
	sy := screen.Bounds().Dy()

	if len(r.Buf) != sx*sy*4 {
		r.Buf = make([]byte, sx*sy*4)
		r.RGBPowerBuf = make([]RGBPower, sx*sy)

		r.tiles = []RenderTile{}
		for tileSrcX := 0; tileSrcX < sx; tileSrcX += r.TileSize {
			for tileSrcY := 0; tileSrcY < sy; tileSrcY += r.TileSize {
				tileSizeX := r.TileSize
				if tileSrcX+r.TileSize > sx {
					tileSizeX = sx - tileSrcX
				}
				tileSizeY := r.TileSize
				if tileSrcY+r.TileSize > sy {
					tileSizeY = sy - tileSrcY
				}
				r.tiles = append(r.tiles, RenderTile{
					SrcX:        tileSrcX,
					SrcY:        tileSrcY,
					LimX:        tileSrcX + tileSizeX,
					LimY:        tileSrcY + tileSizeY,
					RGBPowerBuf: make([]RGBPower, tileSizeX*tileSizeY),
				})
			}
		}
	}

	// The camera's aperture needs to be adjusted to match the screen's ratio.
	//
	// Horizontal FOV is 80 degrees, with vertical FOV adjusted to match.
	r.LatestRenderState.Camera.Aperture[0] = 1
	r.LatestRenderState.Camera.Aperture[1] = fmath.Sin(40.0 / 180.0 * 2.0 * fmath.Pi)
	r.LatestRenderState.Camera.Aperture[2] = r.LatestRenderState.Camera.Aperture[1] / float32(sx) * float32(sy)

	// Render each tile.
	for i := 0; i < len(r.tiles); i++ {
		i := i
		r.renderGroup.Go(func() error {
			tileSizeX := r.tiles[i].LimX - r.tiles[i].SrcX

			for px := r.tiles[i].SrcX; px < r.tiles[i].LimX; px++ {
				for py := r.tiles[i].SrcY; py < r.tiles[i].LimY; py++ {
					tx := px - r.tiles[i].SrcX
					ty := py - r.tiles[i].SrcY
					r.tiles[i].RGBPowerBuf[ty*tileSizeX+tx] = r.LatestRenderState.RenderPixel(px, py, sx, sy)
				}
			}

			// Copy the tile back to the main buf under a lock.
			r.bufLock.Lock()
			defer r.bufLock.Unlock()
			for px := r.tiles[i].SrcX; px < r.tiles[i].LimX; px++ {
				for py := r.tiles[i].SrcY; py < r.tiles[i].LimY; py++ {
					tx := px - r.tiles[i].SrcX
					ty := py - r.tiles[i].SrcY
					r.RGBPowerBuf[py*sx+px] = r.tiles[i].RGBPowerBuf[ty*tileSizeX+tx]
				}
			}

			return nil
		})
	}
	if err := r.renderGroup.Wait(); err != nil {
		log.Printf("Error while rendering: %v", err)
	}

	// Simple automatic exposure control.  Divide all pixels by the maximum RGB
	// power if it's greater than 1.0.
	maxMag := float32(0)
	for px := 0; px < sx; px++ {
		for py := 0; py < sy; py++ {
			thisMag := fmath.Sqrt(r.RGBPowerBuf[py*sx+px][0]*r.RGBPowerBuf[py*sx+px][0] + r.RGBPowerBuf[py*sx+px][1]*r.RGBPowerBuf[py*sx+px][1] + r.RGBPowerBuf[py*sx+px][2]*r.RGBPowerBuf[py*sx+px][2])
			if thisMag > maxMag {
				maxMag = thisMag
			}
		}
	}
	if maxMag < float32(1) {
		maxMag = 1
	}
	exposureDivisor := 1 / maxMag

	for px := 0; px < sx; px++ {
		for py := 0; py < sy; py++ {
			rgbPow := r.RGBPowerBuf[py*sx+px]
			rgbPow[0] *= exposureDivisor
			rgbPow[1] *= exposureDivisor
			rgbPow[2] *= exposureDivisor

			if rgbPow[0] == 1.0 {
				r.Buf[py*sx*4+px*4+0] = 255
			} else {
				r.Buf[py*sx*4+px*4+0] = byte(fmath.Floor(rgbPow[0] * 256.0))
			}

			if rgbPow[1] == 1.0 {
				r.Buf[py*sx*4+px*4+1] = 255
			} else {
				r.Buf[py*sx*4+px*4+1] = byte(fmath.Floor(rgbPow[1] * 256.0))
			}

			if rgbPow[2] == 1.0 {
				r.Buf[py*sx*4+px*4+2] = 255
			} else {
				r.Buf[py*sx*4+px*4+2] = byte(fmath.Floor(rgbPow[2] * 256.0))
			}

			r.Buf[py*sx*4+px*4+3] = 255
		}
	}

	screen.WritePixels(r.Buf)

	// Draw info text
	const x, y = 20, 20
	infoText := fmt.Sprintf("fps=%f\ncurUpscaleFactor=%f\nnewUpscaleFactor=%f", 1.0/drawDelta.Seconds(), r.curUpscaleFactor, newUpscaleFactor)
	w, h := text.Measure(infoText, r.uiNormalFace, r.uiNormalFace.Size*1.5)
	vector.DrawFilledRect(screen, x, y, float32(w), float32(h), color.RGBA{0x80, 0x80, 0x80, 0xff}, false)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.LineSpacing = r.uiNormalFace.Size * 1.5
	text.Draw(screen, infoText, r.uiNormalFace, op)

	r.lastDrawStart = drawStart
}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return int(float64(outsideWidth) / r.curUpscaleFactor), int(float64(outsideHeight) / r.curUpscaleFactor)
}

type RenderState struct {
	Camera *PinholeCamera

	MaxDepth int

	Spheres           []Sphere
	AmbientLights     []AmbientLight
	DirectionalLights []DirectionalLight
	PointLights       []PointLight
}

func (r *RenderState) RenderPixel(px, py, wx, wy int) RGBPower {
	ray := r.Camera.PixelToRay(px, py, wx, wy)

	minContact := Contact{
		Mode: ContactModeMiss,
		T:    100000000.0,
	}
	for i := range r.Spheres {
		if contact := r.Spheres[i].RayContact(ray, 0.0, minContact.T); contact.Mode != ContactModeMiss {
			if contact.T > 0.0 && contact.T < minContact.T {
				minContact = contact
			}
		}
	}

	if minContact.Mode == ContactModeMiss {
		return RGBPower{0.0, 0.0, 0.0}
	}

	power := r.matteShade(&minContact)
	return power
}

func (r *RenderState) matteShade(contact *Contact) RGBPower {
	totalPower := RGBPower{}

	for _, l := range r.AmbientLights {
		totalPower[0] += l.Power[0]
		totalPower[1] += l.Power[1]
		totalPower[2] += l.Power[2]
	}

	for _, l := range r.DirectionalLights {
		dot := fvec3.IProd(contact.N, l.DirectionToSource)
		if dot > 0.0 {
			totalPower[0] += dot * l.Power[0]
			totalPower[1] += dot * l.Power[1]
			totalPower[2] += dot * l.Power[2]
		}
	}

	for _, l := range r.PointLights {
		vToLight := fvec3.SubVV(l.Center, contact.P)
		distanceToLight := vToLight.Norm()
		dirToLight := fvec3.DivVS(vToLight, distanceToLight)
		falloff := 1.0 / distanceToLight / distanceToLight

		dot := fvec3.IProd(contact.N, dirToLight)
		if dot > 0.0 {
			totalPower[0] += dot * falloff * l.Power[0]
			totalPower[1] += dot * falloff * l.Power[1]
			totalPower[2] += dot * falloff * l.Power[2]
		}
	}

	return totalPower
}

type Ray struct {
	Point fvec3.T
	Slope fvec3.T
}

func (r *Ray) Eval(t float32) fvec3.T {
	return fvec3.AddVV(r.Point, fvec3.MulVS(r.Slope, t))
}

type RGBPower [3]float32

type PinholeCamera struct {
	Center          fvec3.T
	ApertureToWorld fmat33.T
	Aperture        fvec3.T
}

func (c *PinholeCamera) PixelToRay(px, py, w, h int /*, rng *rand.Rand*/) Ray {
	imageCoords := fvec3.T{
		1.0,
		1.0 - float32(2)*(float32(px) /*-rng.Float32()*/)/float32(w),
		1.0 - float32(2)*(float32(py) /*-rng.Float32()*/)/float32(h),
	}

	apertureCoords := fvec3.T{
		imageCoords[0] * c.Aperture[0],
		imageCoords[1] * c.Aperture[1],
		imageCoords[2] * c.Aperture[2],
	}

	return Ray{
		Point: c.Center,
		Slope: fvec3.Normalize(fmat33.MulMV(c.ApertureToWorld, apertureCoords)),
	}
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

type ContactMode int

const (
	ContactModeMiss ContactMode = iota
	ContactModeEnter
	ContactModeExit
)

type Contact struct {
	Mode ContactMode

	ExitRay Ray

	T float32
	P fvec3.T
	N fvec3.T

	Mtl0, Mtl1, Mtl2 float32
}

type Sphere struct {
	Center fvec3.T
}

func (s *Sphere) RayContact(ray Ray, queryLo, queryHi float32) Contact {
	b := fvec3.IProd(ray.Slope, ray.Point)
	c := fvec3.IProd(ray.Point, ray.Point) - 1.0

	if b*b-c < 0 {
		return Contact{
			Mode: ContactModeMiss,
		}
	}

	tMin := -b - fmath.Sqrt(b*b-c)
	tMax := -b + fmath.Sqrt(b*b-c)

	if queryLo <= tMin && tMin < queryHi {
		p := ray.Eval(tMin)
		return Contact{
			Mode: ContactModeEnter,

			ExitRay: ray,

			T: tMin,
			P: p,
			N: fvec3.Normalize(p),

			Mtl0: p[0],
			Mtl1: p[1],
			Mtl2: p[2],
		}
	} else {
		p := ray.Eval(tMax)
		return Contact{
			Mode: ContactModeExit,

			ExitRay: ray,

			T: tMin,
			P: p,
			N: fvec3.Normalize(p),

			Mtl0: p[0],
			Mtl1: p[1],
			Mtl2: p[2],
		}
	}
}

type AmbientLight struct {
	Power RGBPower
}

type DirectionalLight struct {
	DirectionToSource fvec3.T
	Power             RGBPower
}

type PointLight struct {
	Center fvec3.T
	Power  RGBPower
}
