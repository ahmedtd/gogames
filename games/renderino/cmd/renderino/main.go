package main

// Dependency on ebitengine pulls in glfw, etc.
//
// For now, run with:
//
//    go run ./games/renderino/cmd/renderino

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"
	"slices"
	"sync"
	"time"

	"shellworld/games/renderino/resources"
	"shellworld/vmath/faffinetransform3"
	"shellworld/vmath/fmat33"
	"shellworld/vmath/fvec3"

	"github.com/barnex/fmath"
	"github.com/google/subcommands"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/sync/errgroup"
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

type Renderer struct {
	PreviousWorldState *WorldState
	LatestWorldState   *WorldState

	LatestRenderState *RenderState

	// Draws per second calculation.  Ebiten's is noisy.
	lastDrawStart time.Time

	// If the framerate is too low, UpscaleFactor will be used to drop the
	// amount of work the raytracer has to do.
	curUpscaleFactor float64

	renderGroup *errgroup.Group

	TileSize int
	tiles    []RenderTile

	renderImage *ebiten.Image
	buf         []byte
	bufLock     sync.Mutex

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

func NewRenderer(numRenderThreads int) (*Renderer, error) {
	camera := &PinholeCamera{
		Center:          fvec3.T{-10, 0, 10},
		ApertureToWorld: fmat33.T{1, 0, 0, 0, 1, 0, 0, 0, 1},
		Aperture:        fvec3.T{0.02, 0.018, 0.012},
	}

	renderGroup := &errgroup.Group{}
	renderGroup.SetLimit(numRenderThreads)

	uiFaceSource, err := text.NewGoTextFaceSource(bytes.NewReader(resources.DepartureMonoOTF))
	if err != nil {
		return nil, fmt.Errorf("while loading Departure Mono font: %w", err)
	}

	staticScene := &RenderState{
		Camera: camera,

		AmbientLights: []AmbientLight{
			{Power: RGBPower{0.2, 0.2, 0.2}},
		},
		DirectionalLights: []DirectionalLight{
			{
				DirectionToSource: fvec3.Normalize(fvec3.T{1, 1, 1}),
				Power:             RGBPower{1.0, 1.0, 1.0},
			},
		},
		PointLights: []PointLight{
			{
				Center: fvec3.T{5, 0, 0},
				Power:  RGBPower{1.0, 0.0, 0.0},
			},
		},
	}

	previousWorldState := &WorldState{}

	cursorX, cursorY := ebiten.CursorPosition()
	previousWorldState.CursorX, previousWorldState.CursorY = cursorX, cursorY

	rng := rand.New(rand.NewPCG(1, 2))

	// Initialize boids
	previousWorldState.BoidRNG = rand.New(rand.NewPCG(1, 2))
	previousWorldState.NextBoidTargetSwitchTime = 10.0
	previousWorldState.BoidTarget = fvec3.UniformSphereDistribution(rng, 100.0)
	previousWorldState.BoidTarget[2] = 0
	previousWorldState.BoidPositions = make([]fvec3.T, 1000)
	previousWorldState.BoidVelocities = make([]fvec3.T, 1000)
	for i := 0; i < 1000; i++ {
		previousWorldState.BoidPositions[i] = fvec3.UniformSphereDistribution(rng, 100.0)
		previousWorldState.BoidPositions[i][2] = 0
		previousWorldState.BoidVelocities[i] = fvec3.UniformSphereDistribution(rng, 1.0)
		previousWorldState.BoidVelocities[i][2] = 0
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

		PreviousWorldState: previousWorldState,
		LatestWorldState:   &WorldState{},

		LatestRenderState: staticScene,
	}

	return r, nil
}

func (r *Renderer) Update() error {
	r.LatestWorldState.SimTime = r.PreviousWorldState.SimTime + (1.0 / 60.0)

	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) {
		r.curUpscaleFactor -= 0.25
		if r.curUpscaleFactor < 1.0 {
			r.curUpscaleFactor = 1.0
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) {
		r.curUpscaleFactor += 0.25
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		go func() {
			f, err := os.Create("renderino-cpu-profile")
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
			f, err := os.Create("renderino-alloc-profile")
			if err != nil {
				log.Printf("Error while opening profile output: %v", err)
				return
			}
			defer f.Close()

			if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
				log.Printf("Error while writing alloc profile: %v", err)
			}
		}()
	}

	r.LatestWorldState.CursorX, r.LatestWorldState.CursorY = ebiten.CursorPosition()

	r.LatestWorldState.BoidRNG = r.PreviousWorldState.BoidRNG
	r.LatestWorldState.NextBoidTargetSwitchTime = r.PreviousWorldState.NextBoidTargetSwitchTime
	r.LatestWorldState.BoidTarget = r.PreviousWorldState.BoidTarget

	if r.PreviousWorldState.SimTime > r.PreviousWorldState.NextBoidTargetSwitchTime {
		r.LatestWorldState.NextBoidTargetSwitchTime = r.PreviousWorldState.SimTime + 10.0
		r.LatestWorldState.BoidTarget = fvec3.UniformSphereDistribution(r.PreviousWorldState.BoidRNG, 100.0)
		r.LatestWorldState.BoidTarget[2] = 0.0
	}

	if len(r.LatestWorldState.BoidPositions) != len(r.PreviousWorldState.BoidPositions) {
		r.LatestWorldState.BoidPositions = make([]fvec3.T, len(r.PreviousWorldState.BoidPositions))
	}
	if len(r.LatestWorldState.BoidVelocities) != len(r.PreviousWorldState.BoidVelocities) {
		r.LatestWorldState.BoidVelocities = make([]fvec3.T, len(r.PreviousWorldState.BoidVelocities))
	}

	target := r.PreviousWorldState.BoidTarget

	// Boids update
	for i := 0; i < len(r.LatestWorldState.BoidPositions); i++ {
		force := fvec3.T{}

		// Every boid is attracted towards the flock target and wants to circle around it.
		offset := fvec3.SubVV(target, r.PreviousWorldState.BoidPositions[i])
		homingForce := offset
		force = fvec3.AddVV(force, homingForce)
		// circForce := fvec3.CProd(offset, fvec3.T{0.0, 0.0, 1.0})
		// force = fvec3.AddVV(force, fvec3.Normalize(circForce))

		// Every boid gets a drag force.
		speed := r.PreviousWorldState.BoidVelocities[i].Norm()
		if speed > 1.0 {
			dragForce := fvec3.MulVS(r.PreviousWorldState.BoidVelocities[i], -1.0*0.1*speed)
			force = fvec3.AddVV(force, dragForce)
		}

		numCohesionInfluencers := 0
		averagePositionOfNeighbors := fvec3.T{}
		numAlignmentInfluencers := 0
		averageVelocityOfNeighbors := fvec3.T{}
		totalSeparationForce := fvec3.T{}
		for j := 0; j < len(r.PreviousWorldState.BoidPositions); j++ {
			// Don't influence yourself.
			if i == j {
				continue
			}

			offset := fvec3.SubVV(r.PreviousWorldState.BoidPositions[i], r.PreviousWorldState.BoidPositions[j])
			separationDistanceSquared := offset[0]*offset[0] + offset[1]*offset[1] + offset[2]*offset[2]

			if separationDistanceSquared < 9.0 {
				numCohesionInfluencers++
				averagePositionOfNeighbors = fvec3.AddVV(averagePositionOfNeighbors, r.PreviousWorldState.BoidPositions[j])
			}

			if separationDistanceSquared < 36.0 {
				numAlignmentInfluencers++
				averageVelocityOfNeighbors = fvec3.AddVV(averageVelocityOfNeighbors, r.PreviousWorldState.BoidVelocities[j])
			}

			if separationDistanceSquared < 4.0 {
				separationHeading := fvec3.Normalize(offset)
				totalSeparationForce = fvec3.AddVV(totalSeparationForce, fvec3.MulVS(separationHeading, 100.0/(1.0+separationDistanceSquared)))
			}
		}

		// Calculate cohesion
		if numCohesionInfluencers != 0 {
			averagePositionOfNeighbors = fvec3.DivVS(averagePositionOfNeighbors, float32(numCohesionInfluencers))
			cohesionOffset := fvec3.SubVV(averagePositionOfNeighbors, r.PreviousWorldState.BoidPositions[i])
			totalCohesionForce := fvec3.MulVS(cohesionOffset, 0.5)
			force = fvec3.AddVV(force, totalCohesionForce)
		}

		// Calculate alignment
		if numAlignmentInfluencers != 0 {
			averageVelocityOfNeighbors = fvec3.DivVS(averageVelocityOfNeighbors, float32(numAlignmentInfluencers))
			totalAlignmentForce := fvec3.MulVS(averageVelocityOfNeighbors, 1.0)
			force = fvec3.AddVV(force, totalAlignmentForce)
		}

		// Apply separation.
		force = fvec3.AddVV(force, totalSeparationForce)

		r.LatestWorldState.BoidVelocities[i][0] = r.PreviousWorldState.BoidVelocities[i][0] + 0.1*force[0]*(1.0/60.0)
		r.LatestWorldState.BoidVelocities[i][1] = r.PreviousWorldState.BoidVelocities[i][1] + 0.1*force[1]*(1.0/60.0)
		r.LatestWorldState.BoidVelocities[i][2] = r.PreviousWorldState.BoidVelocities[i][2] + 0.1*force[2]*(1.0/60.0)

		r.LatestWorldState.BoidPositions[i][0] = r.PreviousWorldState.BoidPositions[i][0] + r.PreviousWorldState.BoidVelocities[i][0]*(1.0/60.0)
		r.LatestWorldState.BoidPositions[i][1] = r.PreviousWorldState.BoidPositions[i][1] + r.PreviousWorldState.BoidVelocities[i][1]*(1.0/60.0)
		r.LatestWorldState.BoidPositions[i][2] = r.PreviousWorldState.BoidPositions[i][2] + r.PreviousWorldState.BoidVelocities[i][2]*(1.0/60.0)
	}

	// TODO: Camera should be tracked in a world state, and we derive the render state from it.
	camStep := float32(1.0 / 8.0)
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		r.LatestRenderState.Camera.Center[0] += r.LatestRenderState.Camera.Eye()[0] * camStep
		r.LatestRenderState.Camera.Center[1] += r.LatestRenderState.Camera.Eye()[1] * camStep
		r.LatestRenderState.Camera.Center[2] += r.LatestRenderState.Camera.Eye()[2] * camStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		r.LatestRenderState.Camera.Center[0] -= r.LatestRenderState.Camera.Eye()[0] * camStep
		r.LatestRenderState.Camera.Center[1] -= r.LatestRenderState.Camera.Eye()[1] * camStep
		r.LatestRenderState.Camera.Center[2] -= r.LatestRenderState.Camera.Eye()[2] * camStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		r.LatestRenderState.Camera.Center[0] += r.LatestRenderState.Camera.Left()[0] * camStep
		r.LatestRenderState.Camera.Center[1] += r.LatestRenderState.Camera.Left()[1] * camStep
		r.LatestRenderState.Camera.Center[2] += r.LatestRenderState.Camera.Left()[2] * camStep
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		r.LatestRenderState.Camera.Center[0] -= r.LatestRenderState.Camera.Left()[0] * camStep
		r.LatestRenderState.Camera.Center[1] -= r.LatestRenderState.Camera.Left()[1] * camStep
		r.LatestRenderState.Camera.Center[2] -= r.LatestRenderState.Camera.Left()[2] * camStep
	}
	cursorDeltaX := r.LatestWorldState.CursorX - r.PreviousWorldState.CursorX
	cursorDeltaY := r.LatestWorldState.CursorY - r.PreviousWorldState.CursorY
	rotateAngleX := float32(cursorDeltaX) * 0.005
	rotateAngleY := float32(cursorDeltaY) * 0.005
	rotateX := faffinetransform3.AxisAngle(fvec3.T{0.0, 0.0, 1.0}, -rotateAngleX)
	rotateY := faffinetransform3.AxisAngle(r.LatestRenderState.Camera.Left(), rotateAngleY)
	r.LatestRenderState.Camera.ApertureToWorld = fmat33.Orthonormalize(fmat33.MulMM(fmat33.MulMM(rotateX.Linear, rotateY.Linear),
		r.LatestRenderState.Camera.ApertureToWorld))

	// TODO: Derive LatestRenderState from LatestWorldState
	r.LatestRenderState.PointLights[0].Center = fvec3.T{
		float32(math.Cos(r.LatestWorldState.SimTime)),
		float32(math.Sin(r.LatestWorldState.SimTime)),
		2.0,
	}

	r.LatestRenderState.AABBs = make([]AABB, len(r.LatestWorldState.BoidPositions))
	for i := 0; i < len(r.LatestWorldState.BoidPositions); i++ {
		r.LatestRenderState.AABBs[i].Lo[0] = r.LatestWorldState.BoidPositions[i][0] - 0.5
		r.LatestRenderState.AABBs[i].Lo[1] = r.LatestWorldState.BoidPositions[i][1] - 0.5
		r.LatestRenderState.AABBs[i].Lo[2] = r.LatestWorldState.BoidPositions[i][2] - 0.5

		r.LatestRenderState.AABBs[i].Hi[0] = r.LatestWorldState.BoidPositions[i][0] + 0.5
		r.LatestRenderState.AABBs[i].Hi[1] = r.LatestWorldState.BoidPositions[i][1] + 0.5
		r.LatestRenderState.AABBs[i].Hi[2] = r.LatestWorldState.BoidPositions[i][2] + 0.5
	}

	// Build tree
	if r.LatestRenderState.Tree == nil {
		r.LatestRenderState.Tree = &BVHTree{}
	}
	r.LatestRenderState.Tree.Clear()
	nodes := []*BVHNode{}
	for i := 0; i < len(r.LatestRenderState.AABBs); i++ {
		node := r.LatestRenderState.Tree.GetFreeNode()
		node.Type = BVHNodeTypeConcrete
		node.Lo = r.LatestRenderState.AABBs[i].Lo
		node.Hi = r.LatestRenderState.AABBs[i].Hi
		node.Ref = i
		nodes = append(nodes, node)
	}
	r.LatestRenderState.Tree.BuildFromElements(nodes)

	r.PreviousWorldState, r.LatestWorldState = r.LatestWorldState, r.PreviousWorldState

	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	drawStart := time.Now()
	drawDelta := drawStart.Sub(r.lastDrawStart)
	if r.lastDrawStart.IsZero() {
		drawDelta = time.Second / 60
	}

	// Compute desired size of the render target according to upscaling factor.
	rx := int(float64(screen.Bounds().Dx()) / r.curUpscaleFactor)
	ry := int(float64(screen.Bounds().Dy()) / r.curUpscaleFactor)

	if r.renderImage == nil || r.renderImage.Bounds().Dx() != rx || r.renderImage.Bounds().Dy() != ry {
		r.renderImage = ebiten.NewImage(rx, ry)
		r.buf = make([]byte, rx*ry*4)
		r.RGBPowerBuf = make([]RGBPower, rx*ry)

		r.tiles = []RenderTile{}
		for tileSrcX := 0; tileSrcX < rx; tileSrcX += r.TileSize {
			for tileSrcY := 0; tileSrcY < ry; tileSrcY += r.TileSize {
				tileSizeX := r.TileSize
				if tileSrcX+r.TileSize > rx {
					tileSizeX = rx - tileSrcX
				}
				tileSizeY := r.TileSize
				if tileSrcY+r.TileSize > ry {
					tileSizeY = ry - tileSrcY
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
	r.LatestRenderState.Camera.Aperture[2] = r.LatestRenderState.Camera.Aperture[1] / float32(rx) * float32(ry)

	// Render each tile.
	for i := 0; i < len(r.tiles); i++ {
		i := i
		r.renderGroup.Go(func() error {
			tileSizeX := r.tiles[i].LimX - r.tiles[i].SrcX

			for px := r.tiles[i].SrcX; px < r.tiles[i].LimX; px++ {
				for py := r.tiles[i].SrcY; py < r.tiles[i].LimY; py++ {
					tx := px - r.tiles[i].SrcX
					ty := py - r.tiles[i].SrcY
					r.tiles[i].RGBPowerBuf[ty*tileSizeX+tx] = r.LatestRenderState.RenderPixel(px, py, rx, ry)
				}
			}

			// Copy the tile back to the main buf under a lock.
			r.bufLock.Lock()
			defer r.bufLock.Unlock()
			for px := r.tiles[i].SrcX; px < r.tiles[i].LimX; px++ {
				for py := r.tiles[i].SrcY; py < r.tiles[i].LimY; py++ {
					tx := px - r.tiles[i].SrcX
					ty := py - r.tiles[i].SrcY
					r.RGBPowerBuf[py*rx+px] = r.tiles[i].RGBPowerBuf[ty*tileSizeX+tx]
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
	drawRenderImageOptions.GeoM.Scale(r.curUpscaleFactor, r.curUpscaleFactor)
	screen.DrawImage(r.renderImage, drawRenderImageOptions)

	cursorX, cursorY := ebiten.CursorPosition()

	// Draw info text
	const x, y = 20, 20
	infoText := fmt.Sprintf("fps=%f\ncurUpscaleFactor=%f\ncursor=%d,%d", 1.0/drawDelta.Seconds(), r.curUpscaleFactor, cursorX, cursorY)
	w, h := text.Measure(infoText, r.uiNormalFace, r.uiNormalFace.Size*1.5)
	vector.DrawFilledRect(screen, x, y, float32(w), float32(h), color.RGBA{0x80, 0x80, 0x80, 0xff}, false)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.LineSpacing = r.uiNormalFace.Size * 1.5
	text.Draw(screen, infoText, r.uiNormalFace, op)

	r.lastDrawStart = drawStart
}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

type WorldState struct {
	SimTime float64

	CursorX, CursorY int

	CameraPosition fvec3.T
	CameraEye      fvec3.T

	BoidRNG                  *rand.Rand
	NextBoidTargetSwitchTime float64
	BoidTarget               fvec3.T
	BoidPositions            []fvec3.T
	BoidVelocities           []fvec3.T
}

type RenderState struct {
	Camera *PinholeCamera

	MaxDepth int

	Spheres           []Sphere
	AABBs             []AABB
	AmbientLights     []AmbientLight
	DirectionalLights []DirectionalLight
	PointLights       []PointLight

	Tree *BVHTree
}

func (r *RenderState) RenderPixel(px, py, wx, wy int) RGBPower {
	ray := r.Camera.PixelToRay(px, py, wx, wy)

	minContact := Contact{
		Mode: ContactModeMiss,
		T:    100000000.0,
	}

	r.Tree.RayTest(ray, func(ref int) {
		contact := r.AABBs[ref].RayContact(ray, 0.0, minContact.T)
		if contact.Mode != ContactModeMiss {
			if contact.T > 0.0 && contact.T < minContact.T {
				minContact = contact
			}
		}
	})

	if minContact.Mode == ContactModeMiss {
		// Render a simple horizon at infinity for orientation
		if ray.Slope[2] < 0.0 {
			return RGBPower{0.0, 0.0, 0.0}
		}
		return RGBPower{0.0, 0.1, 0.5}
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
	Radius float32
}

func (s *Sphere) RayContact(ray Ray, queryLo, queryHi float32) Contact {
	a := ray.Slope[0]*ray.Slope[0] + ray.Slope[1]*ray.Slope[1] + ray.Slope[2]*ray.Slope[2]
	offset := fvec3.SubVV(ray.Point, s.Center)
	b := 2 * (offset[0]*ray.Slope[0] + offset[1]*ray.Slope[1] + offset[2]*ray.Slope[2])
	c := offset[0]*offset[0] + offset[1]*offset[1] + offset[2]*offset[2] - s.Radius*s.Radius

	discriminant := b*b - 4*a*c
	if discriminant < 0 {
		return Contact{
			Mode: ContactModeMiss,
		}
	}

	tMin := -b - fmath.Sqrt(discriminant)/(2*a)
	tMax := -b + fmath.Sqrt(discriminant)/(2*a)

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

type AABB struct {
	Lo fvec3.T
	Hi fvec3.T
}

func (b *AABB) RayContact(ray Ray, queryLo, queryHi float32) Contact {
	tXLo := (b.Lo[0] - ray.Point[0]) / ray.Slope[0]
	tXHi := (b.Hi[0] - ray.Point[0]) / ray.Slope[0]
	tYLo := (b.Lo[1] - ray.Point[1]) / ray.Slope[1]
	tYHi := (b.Hi[1] - ray.Point[1]) / ray.Slope[1]
	tZLo := (b.Lo[2] - ray.Point[2]) / ray.Slope[2]
	tZHi := (b.Hi[2] - ray.Point[2]) / ray.Slope[2]

	centerX := (b.Lo[0] + b.Hi[0]) / 2
	centerY := (b.Lo[1] + b.Hi[1]) / 2
	centerZ := (b.Lo[2] + b.Hi[2]) / 2

	minValidContact := Contact{}

	// Is our contact with the X-Low plane actually on the box?
	pXLoX := b.Lo[0]
	pXLoY := ray.Point[1] + ray.Slope[1]*tXLo
	pXLoZ := ray.Point[2] + ray.Slope[2]*tXLo
	if onBox(pXLoX, pXLoY, pXLoZ, b.Lo[0], b.Hi[0], b.Lo[1], b.Hi[1], b.Lo[2], b.Hi[2]) {
		if (tXLo < minValidContact.T || minValidContact.Mode == ContactModeMiss) && queryLo <= tXLo && tXLo <= queryHi {
			minValidContact.T = tXLo
			if ray.Slope[0] >= 0 {
				minValidContact.Mode = ContactModeEnter
				minValidContact.N = fvec3.T{-1, 0, 0}

			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec3.T{1, 0, 0}
			}
		}
	}

	// Is our contact with the X-High plane actually on the box?
	pXHiX := b.Hi[0]
	pXHiY := ray.Point[1] + ray.Slope[1]*tXHi
	pXHiZ := ray.Point[2] + ray.Slope[2]*tXHi
	if onBox(pXHiX, pXHiY, pXHiZ, b.Lo[0], b.Hi[0], b.Lo[1], b.Hi[1], b.Lo[2], b.Hi[2]) {
		if (tXHi < minValidContact.T || minValidContact.Mode == ContactModeMiss) && queryLo <= tXHi && tXHi <= queryHi {
			minValidContact.T = tXHi
			if ray.Slope[0] < 0 {
				minValidContact.Mode = ContactModeEnter
				minValidContact.N = fvec3.T{1, 0, 0}
			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec3.T{-1, 0, 0}
			}
		}
	}

	// Is our contact with the Y-Low plane actually on the box?
	pYLoX := ray.Point[0] + ray.Slope[0]*tYLo
	pYLoY := b.Lo[1]
	pYLoZ := ray.Point[2] + ray.Slope[2]*tYLo
	if onBox(pYLoX, pYLoY, pYLoZ, b.Lo[0], b.Hi[0], b.Lo[1], b.Hi[1], b.Lo[2], b.Hi[2]) {
		if (tYLo < minValidContact.T || minValidContact.Mode == ContactModeMiss) && queryLo <= tYLo && tYLo <= queryHi {
			minValidContact.T = tYLo
			if ray.Slope[1] >= 0 {
				minValidContact.Mode = ContactModeEnter
				minValidContact.N = fvec3.T{0, -1, 0}
			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec3.T{0, 1, 0}
			}
		}
	}

	// Is our contact with the Y-High plane actually on the box?
	pYHiX := ray.Point[0] + ray.Slope[0]*tYHi
	pYHiY := b.Hi[1]
	pYHiZ := ray.Point[2] + ray.Slope[2]*tYHi
	if onBox(pYHiX, pYHiY, pYHiZ, b.Lo[0], b.Hi[0], b.Lo[1], b.Hi[1], b.Lo[2], b.Hi[2]) {
		if (tYHi < minValidContact.T || minValidContact.Mode == ContactModeMiss) && queryLo <= tYHi && tYHi <= queryHi {
			minValidContact.T = tYHi
			if ray.Slope[1] < 0 {
				minValidContact.Mode = ContactModeEnter
				minValidContact.N = fvec3.T{0, 1, 0}
			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec3.T{0, -1, 0}
			}
		}
	}

	// Is our contact with the Z-Low plane actually on the box?
	pZLoX := ray.Point[0] + ray.Slope[0]*tZLo
	pZLoY := ray.Point[1] + ray.Slope[1]*tZLo
	pZLoZ := b.Lo[2]
	if onBox(pZLoX, pZLoY, pZLoZ, b.Lo[0], b.Hi[0], b.Lo[1], b.Hi[1], b.Lo[2], b.Hi[2]) {
		if (tZLo < minValidContact.T || minValidContact.Mode == ContactModeMiss) && queryLo <= tZLo && tZLo <= queryHi {
			minValidContact.T = tZLo
			if ray.Slope[2] >= 0 {
				minValidContact.Mode = ContactModeEnter
				minValidContact.N = fvec3.T{0, 0, -1}
			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec3.T{0, 0, 1}
			}
		}
	}

	// Is our contact with the Z-High plane actually on the box?
	pZHiX := ray.Point[0] + ray.Slope[0]*tZHi
	pZHiY := ray.Point[1] + ray.Slope[1]*tZHi
	pZHiZ := b.Hi[2]
	if onBox(pZHiX, pZHiY, pZHiZ, b.Lo[0], b.Hi[0], b.Lo[1], b.Hi[1], b.Lo[2], b.Hi[2]) {
		if (tZHi < minValidContact.T || minValidContact.Mode == ContactModeMiss) && queryLo <= tZHi && tZHi <= queryHi {
			minValidContact.T = tZHi
			if ray.Slope[2] < 0 {
				minValidContact.Mode = ContactModeEnter
				minValidContact.N = fvec3.T{0, 0, 1}
			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec3.T{0, 0, -1}
			}
		}
	}

	minValidContact.Mtl0 = minValidContact.P[0] - centerX
	minValidContact.Mtl1 = minValidContact.P[1] - centerY
	minValidContact.Mtl2 = minValidContact.P[2] - centerZ

	return minValidContact
}

func onBox(px, py, pz, xlo, xhi, ylo, yhi, zlo, zhi float32) bool {
	return xlo <= px && px <= xhi && ylo <= py && py <= yhi && zlo <= pz && pz <= zhi
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

type BVHTree struct {
	Root *BVHNode

	FreeList                  []*BVHNode
	AllocationsSinceLastClear int
}

// Clear moves all nodes from the tree to the free list.
func (t *BVHTree) Clear() {
	//freeListLenBeforeClear := len(t.FreeList)

	workStack := []*BVHNode{t.Root}
	for len(workStack) != 0 {
		cur := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		if cur == nil {
			continue
		}

		workStack = append(workStack, cur.LoChild)
		workStack = append(workStack, cur.HiChild)
		*cur = BVHNode{}

		t.FreeList = append(t.FreeList, cur)
	}

	//log.Printf("allocationsSinceLastClear=%d freeListLenBefore=%d freeListLenAfter=%d", t.AllocationsSinceLastClear, freeListLenBeforeClear, len(t.FreeList))
	t.AllocationsSinceLastClear = 0

	t.Root = nil
}

func (t *BVHTree) GetFreeNode() *BVHNode {
	if len(t.FreeList) == 0 {
		t.AllocationsSinceLastClear++
		return &BVHNode{}
	}

	picked := t.FreeList[len(t.FreeList)-1]
	t.FreeList = t.FreeList[:len(t.FreeList)-1]

	return picked
}

// RayTest fires a ray through the BVH tree, invoking checkSceneElement with the
// index of any scene element that could be hit.
func (t *BVHTree) RayTest(ray Ray, checkRef func(ref int)) {
	inverseSlope := fvec3.T{
		1.0 / ray.Slope[0],
		1.0 / ray.Slope[1],
		1.0 / ray.Slope[2],
	}

	workStack := make([]*BVHNode, 0, 32)
	workStack = append(workStack, t.Root)
	for len(workStack) != 0 {
		node := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		if node == nil {
			continue
		}

		// If we didn't hit this node, we don't need to look at it any further.
		if !node.RayTest(ray.Point, inverseSlope) {
			continue
		}

		switch node.Type {
		case BVHNodeTypeConcrete:
			checkRef(node.Ref)
		case BVHNodeTypeComposite:
			workStack = append(workStack, node.LoChild, node.HiChild)
		}
	}
}

type BVHNodeType int

const (
	BVHNodeTypeConcrete BVHNodeType = iota
	BVHNodeTypeComposite
)

type BVHNode struct {
	Type BVHNodeType
	Lo   fvec3.T
	Hi   fvec3.T

	Ref int

	LoChild *BVHNode
	HiChild *BVHNode
}

func (b *BVHNode) RayTest(point, inverseSlope fvec3.T) bool {
	// https://tavianator.com/2011/ray_box.html

	tx1 := (b.Lo[0] - point[0]) * inverseSlope[0]
	tx2 := (b.Hi[0] - point[0]) * inverseSlope[0]

	tMin := min(tx1, tx2)
	tMax := max(tx1, tx2)

	ty1 := (b.Lo[1] - point[1]) * inverseSlope[1]
	ty2 := (b.Hi[1] - point[1]) * inverseSlope[1]

	tMin = max(tMin, min(ty1, ty2))
	tMax = min(tMax, max(ty1, ty2))

	tz1 := (b.Lo[2] - point[2]) * inverseSlope[2]
	tz2 := (b.Hi[2] - point[2]) * inverseSlope[2]

	tMin = max(tMin, min(tz1, tz2))
	tMax = min(tMax, max(tz1, tz2))

	return tMin <= tMax
}

func (t *BVHTree) BuildFromElements(elements []*BVHNode) {
	t.Root = t.assembleBVHTree(elements, 0)
}

func (t *BVHTree) assembleBVHTree(elements []*BVHNode, axis int) *BVHNode {
	// Base case.
	if len(elements) == 0 {
		return nil
	}

	// Base case.  Make root a concrete node that holds only this elements.
	if len(elements) == 1 {
		if elements[0].Type != BVHNodeTypeConcrete {
			panic("composite BVH node passed to assembleBVHTree")
		}
		return elements[0]
	}

	// Recursive case.  Pick a reasonable split and divide elements into a high
	// and low child of the current node.

	// Sort all elements according to the split axis.
	slices.SortFunc[[]*BVHNode, *BVHNode](elements, func(a, b *BVHNode) int {
		diff := a.Lo[axis] - b.Lo[axis]
		switch {
		case diff < 0:
			return -1
		case diff == 0:
			return 0
		default:
			return 1
		}
	})

	// Split in the middle of the list.
	splitIndex := len(elements) / 2
	loChild := t.assembleBVHTree(elements[0:splitIndex], (axis+1)%3)
	hiChild := t.assembleBVHTree(elements[splitIndex:], (axis+1)%3)

	boundsLo := fvec3.T{}
	boundsLo[0] = min(loChild.Lo[0], hiChild.Lo[0])
	boundsLo[1] = min(loChild.Lo[1], hiChild.Lo[1])
	boundsLo[2] = min(loChild.Lo[2], hiChild.Lo[2])

	boundsHi := fvec3.T{}
	boundsHi[0] = max(loChild.Hi[0], hiChild.Hi[0])
	boundsHi[1] = max(loChild.Hi[1], hiChild.Hi[1])
	boundsHi[2] = max(loChild.Hi[2], hiChild.Hi[2])

	node := t.GetFreeNode()
	node.Type = BVHNodeTypeComposite
	node.Lo = boundsLo
	node.Hi = boundsHi
	node.LoChild = loChild
	node.HiChild = hiChild

	return node
}
