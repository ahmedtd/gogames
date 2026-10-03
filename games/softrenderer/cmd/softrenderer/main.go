package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image/color"
	"log"
	"math"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"
	"time"

	"github.com/ahmedtd/gogames/embeddedassets/departuremonofont"
	"github.com/ahmedtd/gogames/embeddedassets/kenneycastlekit"
	"github.com/ahmedtd/gogames/lib/graphics/renderable3"
	"github.com/ahmedtd/gogames/lib/graphics/scene3"
	"github.com/ahmedtd/gogames/lib/vmath/fmat44"
	"github.com/ahmedtd/gogames/lib/vmath/fvec2"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"

	"github.com/google/subcommands"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
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
	Vs            [3]fvec3.T
	TextureCoords [3]fvec2.T

	Diffuse *renderable3.UncompressedImage
}

type Renderer struct {
	frameCounter int

	renderImage *ebiten.Image
	buf         []byte

	// Draws per second calculation.  Ebiten's is noisy.
	lastDrawStart time.Time

	RenderScene   *scene3.Scene
	DisplayTarget *scene3.RenderTarget

	CamPos fvec3.T

	Meshes map[string]*renderable3.T

	UIFaceSource *text.GoTextFaceSource
	UINormalFace *text.GoTextFace
}

var _ ebiten.Game = &Renderer{}

func NewRenderer(numRenderThreads int) (*Renderer, error) {

	ballistaMesh, err := kenneycastlekit.LoadSiegeBallista(false)
	if err != nil {
		return nil, fmt.Errorf("while loading ballista mesh: %w", err)
	}

	uiFaceSource, err := text.NewGoTextFaceSource(bytes.NewReader(departuremonofont.DepartureMonoOTF))
	if err != nil {
		return nil, fmt.Errorf("while loading Departure Mono font: %w", err)
	}

	r := &Renderer{
		RenderScene: &scene3.Scene{
			MeshInstances: []*scene3.MeshInstance{
				{
					Mesh:         ballistaMesh,
					ModelToWorld: fmat44.Identity(),
				},
			},
		},
		Meshes: map[string]*renderable3.T{
			"kenneycastlekit/siege-ballista": ballistaMesh,
		},

		UIFaceSource: uiFaceSource,
		UINormalFace: &text.GoTextFace{
			Source: uiFaceSource,
			Size:   24,
		},
	}
	return r, nil
}

func (r *Renderer) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		go func() {
			f, err := os.Create("cpu.pprof")
			if err != nil {
				log.Printf("Error while opening profile output: %v", err)
				return
			}
			defer f.Close()

			if err := pprof.StartCPUProfile(f); err != nil {
				log.Printf("Error while starting CPU profile: %v", err)
				return
			}
			defer pprof.StopCPUProfile()
			time.Sleep(10 * time.Second)
		}()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF4) {
		go func() {
			f, err := os.Create("heap.pprof")
			if err != nil {
				log.Printf("Error while opening profile output: %v", err)
				return
			}
			defer f.Close()

			if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil {
				log.Printf("Error while writing heap profile: %v", err)
			}
		}()
	}

	angle0 := ((float64(r.frameCounter) / 120 * 30.0) + 0.0) * math.Pi / 180.0

	r.CamPos[0] = float32(math.Cos(angle0)) * 20.0
	r.CamPos[1] = float32(math.Sin(angle0)) * 20.0
	r.CamPos[2] = 10.0

	r.frameCounter++

	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	drawStart := time.Now()
	drawDelta := drawStart.Sub(r.lastDrawStart)
	if r.lastDrawStart.IsZero() {
		drawDelta = time.Second / 60
	}

	// Compute desired size of the render target according to upscaling factor.
	rx := int(float64(screen.Bounds().Dx()) / 1)
	ry := int(float64(screen.Bounds().Dy()) / 1)

	if r.DisplayTarget == nil || r.DisplayTarget.Width != rx || r.DisplayTarget.Height != ry {
		r.DisplayTarget = scene3.MakeRenderTarget(rx, ry)
		r.buf = make([]byte, rx*ry*4)
		r.renderImage = ebiten.NewImage(rx, ry)
	}

	for i := 0; i < rx*ry; i++ {
		r.buf[i*4+0] = 0
		r.buf[i*4+1] = 0
		r.buf[i*4+2] = 0
		r.buf[i*4+3] = 255
	}

	r.DisplayTarget.Clear()

	// Compose camera transform
	camMatrix := fmat44.MulMM(
		fmat44.Perspective(3),
		fmat44.LookAt(r.CamPos, fvec3.T{}, fvec3.T{0, 0, 1}),
	)

	r.RenderScene.Render(camMatrix, r.DisplayTarget)

	r.DisplayTarget.QuantizeColor(r.buf)
	r.renderImage.WritePixels(r.buf)

	drawRenderImageOptions := &ebiten.DrawImageOptions{
		Filter: ebiten.FilterNearest,
	}
	screen.DrawImage(r.renderImage, drawRenderImageOptions)

	// Draw info text
	const x, y = 20, 20
	infoText := fmt.Sprintf("fps=%f", 1.0/drawDelta.Seconds())
	w, h := text.Measure(infoText, r.UINormalFace, r.UINormalFace.Size*1.5)
	vector.DrawFilledRect(screen, x, y, float32(w), float32(h), color.RGBA{0x80, 0x80, 0x80, 0xff}, false)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.LineSpacing = r.UINormalFace.Size * 1.5
	text.Draw(screen, infoText, r.UINormalFace, op)

	r.lastDrawStart = drawStart
}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}
