package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/ahmedtd/gogames/lib/vmath/faabox2"
	"github.com/ahmedtd/gogames/lib/vmath/fvec2"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
	"github.com/barnex/fmath"

	"github.com/google/subcommands"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")

	subcommands.Register(&RunCommand{}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}

type RunCommand struct {
	numRenderThreads int
}

var _ subcommands.Command = (*RunCommand)(nil)

func (c *RunCommand) Name() string {
	return "run"
}

func (c *RunCommand) Synopsis() string {
	return "run"
}

func (c *RunCommand) Usage() string {
	return ""
}

func (c *RunCommand) SetFlags(f *flag.FlagSet) {
	f.IntVar(&c.numRenderThreads, "num-render-threads", 0, "How many render threads should be used?")
}

func (c *RunCommand) Execute(ctx context.Context, f *flag.FlagSet, args ...any) subcommands.ExitStatus {
	if err := c.executeError(ctx, f, args...); err != nil {
		log.Printf("Error: %v", err)
		return subcommands.ExitFailure
	}

	return subcommands.ExitSuccess
}

func (c *RunCommand) executeError(ctx context.Context, f *flag.FlagSet, args ...any) error {
	go func() {
		log.Println(http.ListenAndServe("localhost:8080", nil))
	}()

	renderer, err := NewRenderer(c.numRenderThreads)
	if err != nil {
		return fmt.Errorf("while creating renderer: %w", err)
	}

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(1024, 768)
	ebiten.SetWindowTitle("renderino")

	ebiten.SetCursorMode(ebiten.CursorModeCaptured)

	if err := ebiten.RunGame(renderer); err != nil {
		return fmt.Errorf("while running game: %w", err)
	}

	return nil
}

type RGBPower [3]float32

type Triangle struct {
	Vs [3]fvec3.T
}

type Renderer struct {
	frameCounter int

	xSize, ySize int

	renderImage *ebiten.Image
	buf         []byte

	RGBPowerBuf []RGBPower

	Objs []Triangle
}

var _ ebiten.Game = &Renderer{}

func NewRenderer(numRenderThreads int) (*Renderer, error) {
	r := &Renderer{
		Objs: make([]Triangle, 1),
	}
	return r, nil
}

func (r *Renderer) Update() error {
	angle0 := ((float64(r.frameCounter) / 120 * 30.0) + 0.0) * math.Pi / 180.0
	angle1 := ((float64(r.frameCounter) / 120 * 30.0) + 150.0) * math.Pi / 180.0
	angle2 := ((float64(r.frameCounter) / 120 * 30.0) + 210.0) * math.Pi / 180.0

	r.Objs[0].Vs[0][0] = float32(math.Cos(angle0))*100 + 100
	r.Objs[0].Vs[0][1] = float32(math.Sin(angle0))*100 + 100

	r.Objs[0].Vs[1][0] = float32(math.Cos(angle1))*100 + 100
	r.Objs[0].Vs[1][1] = float32(math.Sin(angle1))*100 + 100

	r.Objs[0].Vs[2][0] = float32(math.Cos(angle2))*100 + 100
	r.Objs[0].Vs[2][1] = float32(math.Sin(angle2))*100 + 100

	r.frameCounter++

	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	// Compute desired size of the render target according to upscaling factor.
	rx := int(float64(screen.Bounds().Dx()) / 1)
	ry := int(float64(screen.Bounds().Dy()) / 1)
	r.xSize = rx
	r.ySize = ry

	if r.renderImage == nil || r.renderImage.Bounds().Dx() != rx || r.renderImage.Bounds().Dy() != ry {
		r.renderImage = ebiten.NewImage(rx, ry)
		r.buf = make([]byte, rx*ry*4)
		r.RGBPowerBuf = make([]RGBPower, rx*ry)
	}

	for i := 0; i < len(r.buf)/4; i++ {
		r.buf[i*4+0] = 0
		r.buf[i*4+1] = 0
		r.buf[i*4+2] = 0
		r.buf[i*4+3] = 255
	}

	for _, tri := range r.Objs {
		r.DrawTriangle(tri)
	}

	r.renderImage.WritePixels(r.buf)

	drawRenderImageOptions := &ebiten.DrawImageOptions{
		Filter: ebiten.FilterLinear,
	}
	drawRenderImageOptions.GeoM.Scale(1.0, 1.0)
	screen.DrawImage(r.renderImage, drawRenderImageOptions)
}

func (r *Renderer) DrawTriangle(tri Triangle) {
	// Convert to screen space.  Will eventually be an affine transform and
	// perspective divide.
	sp0 := fvec2.T{tri.Vs[0][0], tri.Vs[0][1]}
	sp1 := fvec2.T{tri.Vs[1][0], tri.Vs[1][1]}
	sp2 := fvec2.T{tri.Vs[2][0], tri.Vs[2][1]}

	// Note: Cannot just follow the guide here.  We need to do some fancy stuff
	// to make sure that interpolation of texture coordinates in screen space is
	// perspective-correct. See
	// https://www.cs.cornell.edu/~srm/fcg3/updated-11.3.1.pdf

	totalArea := signedTriangleArea(sp0, sp1, sp2)

	bbox := faabox2.OfPoints(sp0, sp1, sp2)
	bbox.Lo[0] = fmath.Floor(bbox.Lo[0])
	bbox.Lo[1] = fmath.Floor(bbox.Lo[1])
	bbox.Hi[0] = fmath.Ceil(bbox.Hi[0])
	bbox.Hi[1] = fmath.Ceil(bbox.Hi[1])

	for x := int(bbox.Lo[0]); x < int(bbox.Hi[0]); x++ {
		for y := int(bbox.Lo[1]); y < int(bbox.Hi[1]); y++ {
			if x < 0 || x >= r.xSize || y < 0 || y >= r.ySize {
				continue
			}

			fragCenter := fvec2.T{float32(x), float32(y)}

			b0 := signedTriangleArea(fragCenter, sp1, sp2) / totalArea
			b1 := signedTriangleArea(fragCenter, sp2, sp0) / totalArea
			b2 := signedTriangleArea(fragCenter, sp0, sp1) / totalArea

			if b0 < 0 || b1 < 0 || b2 < 0 {
				continue
			}

			r.buf[y*r.xSize*4+x*4+0] = 255
			r.buf[y*r.xSize*4+x*4+1] = 0
			r.buf[y*r.xSize*4+x*4+2] = 0
			r.buf[y*r.xSize*4+x*4+3] = 255
		}
	}

}

func signedTriangleArea(a, b, c fvec2.T) float32 {
	ab := fvec2.Sub(b, a)
	ac := fvec2.Sub(c, a)
	return 0.5 * (ab[0]*ac[1] - ab[1]*ac[0])
}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}
