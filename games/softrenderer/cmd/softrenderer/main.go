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

	"github.com/ahmedtd/gogames/embeddedassets/kenneycastlekit"
	"github.com/ahmedtd/gogames/lib/objloader"
	"github.com/ahmedtd/gogames/lib/vmath/fmat44"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
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
	DepthBuf    []float32

	Objs   []Triangle
	CamPos fvec3.T

	Meshes map[string]*objloader.Mesh
}

var _ ebiten.Game = &Renderer{}

func NewRenderer(numRenderThreads int) (*Renderer, error) {

	ballistaMesh, err := kenneycastlekit.LoadSiegeBallista()
	if err != nil {
		return nil, fmt.Errorf("while loading ballista mesh: %w", err)
	}

	r := &Renderer{
		Objs: make([]Triangle, 1),
		Meshes: map[string]*objloader.Mesh{
			"kenneycastlekit/siege-ballista": ballistaMesh,
		},
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

	r.CamPos[0] = float32(math.Cos(angle0)) * 20.0
	r.CamPos[1] = float32(math.Sin(angle0)) * 20.0
	r.CamPos[2] = 10.0

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
		r.DepthBuf = make([]float32, rx*ry)
	}

	for i := 0; i < r.xSize*r.ySize; i++ {
		r.RGBPowerBuf[i] = RGBPower{0, 0, 0}
		r.DepthBuf[i] = float32(math.Inf(-1))

		r.buf[i*4+0] = 0
		r.buf[i*4+1] = 0
		r.buf[i*4+2] = 0
		r.buf[i*4+3] = 255
	}

	// Compose camera transform
	camMatrix := fmat44.MulMM(
		fmat44.Perspective(3),
		fmat44.LookAt(r.CamPos, fvec3.T{}, fvec3.T{0, 0, 1}),
	)

	viewportMatrix := fmat44.Viewport(float32(rx), float32(ry))

	r.DrawMesh(camMatrix, viewportMatrix, "kenneycastlekit/siege-ballista")

	// Simple automatic exposure control.  Divide all pixels by the maximum RGB
	// power if it's greater than 1.0.
	maxMag := float32(0)
	for px := 0; px < rx; px++ {
		for py := 0; py < ry; py++ {
			thisMag := fmath.Sqrt(r.RGBPowerBuf[py*rx+px][0]*r.RGBPowerBuf[py*rx+px][0] + r.RGBPowerBuf[py*rx+px][1]*r.RGBPowerBuf[py*rx+px][1] + r.RGBPowerBuf[py*rx+px][2]*r.RGBPowerBuf[py*rx+px][2])
			if thisMag > maxMag {
				maxMag = thisMag
			}
		}
	}
	if maxMag < float32(1) {
		maxMag = 1
	}
	exposureDivisor := 1 / maxMag

	for px := 0; px < rx; px++ {
		for py := 0; py < ry; py++ {
			rgbPow := r.RGBPowerBuf[py*rx+px]
			rgbPow[0] *= exposureDivisor
			rgbPow[1] *= exposureDivisor
			rgbPow[2] *= exposureDivisor

			if rgbPow[0] == 1.0 {
				r.buf[py*rx*4+px*4+0] = 255
			} else {
				r.buf[py*rx*4+px*4+0] = byte(fmath.Floor(rgbPow[0] * 256.0))
			}

			if rgbPow[1] == 1.0 {
				r.buf[py*rx*4+px*4+1] = 255
			} else {
				r.buf[py*rx*4+px*4+1] = byte(fmath.Floor(rgbPow[1] * 256.0))
			}

			if rgbPow[2] == 1.0 {
				r.buf[py*rx*4+px*4+2] = 255
			} else {
				r.buf[py*rx*4+px*4+2] = byte(fmath.Floor(rgbPow[2] * 256.0))
			}

			r.buf[py*rx*4+px*4+3] = 255
		}
	}

	r.renderImage.WritePixels(r.buf)

	drawRenderImageOptions := &ebiten.DrawImageOptions{
		Filter: ebiten.FilterLinear,
	}
	drawRenderImageOptions.GeoM.Scale(1.0, 1.0)
	screen.DrawImage(r.renderImage, drawRenderImageOptions)
}

func (r *Renderer) DrawMesh(camMatrix, viewportMatrix fmat44.T, meshName string) {
	mesh := r.Meshes[meshName]

	for _, tri := range mesh.Tris {
		concreteTri := Triangle{}
		for i := range 3 {
			ind := tri.VertexIndices[i]
			concreteTri.Vs[i] = mesh.Vertices[ind]
		}
		r.DrawTriangle(camMatrix, viewportMatrix, concreteTri)
	}
}

func (r *Renderer) DrawTriangle(camMatrix, viewportMatrix fmat44.T, tri Triangle) {
	// Compute normalized device coordinates for the triangle.
	var ndcs [3]fvec4.T
	for i := range 3 {
		ndcs[i] = fmat44.MulMV(camMatrix, fvec4.T{tri.Vs[i][0], tri.Vs[i][1], tri.Vs[i][2], 1.0})
		ndcs[i][3] = 1
	}

	var viewportCoords [3]fvec4.T
	for i := range 3 {
		viewportCoords[i] = fmat44.MulMV(viewportMatrix, ndcs[i])
	}

	color0 := RGBPower{1, 0, 0}
	color1 := RGBPower{0, 1, 0}
	color2 := RGBPower{0, 0, 1}

	// Note: Cannot just follow the guide here.  We need to do some fancy stuff
	// to make sure that interpolation of texture coordinates in screen space is
	// perspective-correct. See
	// https://www.cs.cornell.edu/~srm/fcg3/updated-11.3.1.pdf

	totalArea := signedTriangleArea(viewportCoords[0], viewportCoords[1], viewportCoords[2])

	if totalArea <= 0 {
		return
	}

	// Compute a screen-coordinate bounding box
	minX := float32(math.Inf(1))
	minY := float32(math.Inf(1))
	maxX := float32(math.Inf(-1))
	maxY := float32(math.Inf(-1))
	for i := 0; i < 3; i++ {
		if viewportCoords[i][0] < minX {
			minX = viewportCoords[i][0]
		}
		if viewportCoords[i][0] > maxX {
			maxX = viewportCoords[i][0]
		}
		if viewportCoords[i][1] < minY {
			minY = viewportCoords[i][1]
		}
		if viewportCoords[i][1] > maxY {
			maxY = viewportCoords[i][1]
		}
	}
	minX = fmath.Floor(minX)
	maxX = fmath.Ceil(maxX)
	minY = fmath.Floor(minY)
	maxY = fmath.Ceil(maxY)

	for x := int(minX); x < int(maxX); x++ {
		for y := int(minY); y < int(maxY); y++ {
			if x < 0 || x >= r.xSize || y < 0 || y >= r.ySize {
				continue
			}

			fragCenter := fvec4.T{float32(x), float32(y), 0, 1}

			b0 := signedTriangleArea(fragCenter, viewportCoords[1], viewportCoords[2]) / totalArea
			b1 := signedTriangleArea(fragCenter, viewportCoords[2], viewportCoords[0]) / totalArea
			b2 := signedTriangleArea(fragCenter, viewportCoords[0], viewportCoords[1]) / totalArea

			if b0 < 0 || b1 < 0 || b2 < 0 {
				continue
			}

			// Depth test
			fragDepth := b0*viewportCoords[0][2] + b1*viewportCoords[1][2] + b2*viewportCoords[2][2]
			if fragDepth <= r.DepthBuf[y*r.xSize+x] {
				continue
			}
			r.DepthBuf[y*r.xSize+x] = fragDepth

			for i := 0; i < 3; i++ {
				r.RGBPowerBuf[y*r.xSize+x][i] = b0*color0[i] + b1*color1[i] + b2*color2[i]
			}

		}
	}

}

func signedTriangleArea(a, b, c fvec4.T) float32 {
	ab := fvec4.SubVV(b, a)
	ac := fvec4.SubVV(c, a)
	return 0.5 * (ab[0]*ac[1] - ab[1]*ac[0])
}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}
