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
	"time"

	"github.com/ahmedtd/gogames/embeddedassets/departuremonofont"
	"github.com/ahmedtd/gogames/embeddedassets/kenneycastlekit"
	"github.com/ahmedtd/gogames/games/renderino/internal/bvh"
	"github.com/ahmedtd/gogames/lib/vmath/fmat44"
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"

	"github.com/barnex/fmath"
	"github.com/google/subcommands"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/sync/errgroup"
)

// TODO: Profiling shows that most time is being taken in PixelToRay and BVH
// RayTest.
//
// PixelToRay can probably be accelarated by implementing it in terms of
// archsimd.Float32x4, as well as unrolling it across N SIMD registers. Instead
// of calculating PixelToRay once for each pixel, we should first compose an
// index buffer in the RenderTile, then use SIMD to convert that to a ray buffer
// very quickly.  (Similar to denseDot2 in go-ml)
//
// BVH RayTest will be more difficult.  It might be possible to query multiple
// nodes simultaneously.  Each node could potentially have 8 children

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
	tiles    []*RenderTile

	renderImage *ebiten.Image
	buf         []byte

	uiFaceSource *text.GoTextFaceSource
	uiNormalFace *text.GoTextFace
}

type RenderTile struct {
	SrcX, SrcY  int
	LimX, LimY  int
	RGBPowerBuf []RGBPower

	// Histogram of pixel magnitudes in RGBPowerBuf, from the most recent
	// render.  See exposureHistogramBin.
	MagHistogram [exposureHistogramBins]int32

	// Size (LimX-SrcX)*(LimY-SrcY)
	IndexBuf   [][4]float32
	CameraRays [][4]float32
}

var _ ebiten.Game = &Renderer{}

func NewRenderer(numRenderThreads int) (*Renderer, error) {
	camera := &PinholeCamera{
		Center:          fvec4.T{-10, 0, 10},
		ApertureToWorld: fmat44.Identity(),
		Aperture:        fvec4.T{0.02, 0.018, 0.012},
	}

	renderGroup := &errgroup.Group{}
	renderGroup.SetLimit(numRenderThreads)

	uiFaceSource, err := text.NewGoTextFaceSource(bytes.NewReader(departuremonofont.DepartureMonoOTF))
	if err != nil {
		return nil, fmt.Errorf("while loading Departure Mono font: %w", err)
	}

	ballistaMesh, err := kenneycastlekit.LoadSiegeBallista(true)
	if err != nil {
		return nil, fmt.Errorf("while loading ballista mesh: %w", err)
	}
	ballistaModel := NewMeshModel(ballistaMesh)

	var ballistas []MeshInstance
	for i := -1; i <= 1; i++ {
		modelToWorld := fmat44.MulMM(
			translateMat44(fvec4.T{0, 8 * float32(i), 0}),
			fmat44.MulMM(
				axisAngleMat44(fvec4.T{0, 0, 1}, float32(i)*fmath.Pi/4),
				scaleMat44(4),
			),
		)
		ballistas = append(ballistas, NewMeshInstance(ballistaModel, modelToWorld))
	}

	staticScene := &RenderState{
		Camera: camera,

		MeshInstances: ballistas,

		AmbientLights: []AmbientLight{
			{Power: RGBPower{0.2, 0.2, 0.2}},
		},
		DirectionalLights: []DirectionalLight{
			{
				DirectionToSource: fvec4.Normalize(fvec4.T{1, 1, 1}),
				Power:             RGBPower{1.0, 1.0, 1.0},
			},
		},
		PointLights: []PointLight{
			{
				Center: fvec4.T{5, 0, 0},
				Power:  RGBPower{10.0, 0.0, 0.0},
			},
		},
	}

	previousWorldState := &WorldState{}

	cursorX, cursorY := ebiten.CursorPosition()
	previousWorldState.CursorX, previousWorldState.CursorY = cursorX, cursorY

	rng := rand.New(rand.NewPCG(1, 2))

	// Initialize boids.  Each boid is one box in the scene, so this sets how
	// hard the renderer is taxed; the O(n^2) boid update is serial, so keep it
	// from dominating frame time.
	const numBoids = 500
	previousWorldState.BoidRNG = rand.New(rand.NewPCG(1, 2))
	previousWorldState.NextBoidTargetSwitchTime = 10.0
	previousWorldState.BoidTarget = fvec4.UniformSphereDistribution(rng, 100.0)
	previousWorldState.BoidTarget[2] = 0
	previousWorldState.BoidPositions = make([]fvec4.T, numBoids)
	previousWorldState.BoidVelocities = make([]fvec4.T, numBoids)
	for i := 0; i < numBoids; i++ {
		previousWorldState.BoidPositions[i] = fvec4.UniformSphereDistribution(rng, 100.0)
		previousWorldState.BoidPositions[i][2] = 0
		previousWorldState.BoidVelocities[i] = fvec4.UniformSphereDistribution(rng, 1.0)
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

// axisAngleMat44 builds a 4x4 rotation matrix (a 3x3 rotation embedded in the
// top-left block, with the last row/column set to the identity) that rotates
// by angle radians around axis u. u's w component is ignored.
func axisAngleMat44(u fvec4.T, angle float32) fmat44.T {
	c := fmath.Cos(angle)
	s := fmath.Sin(angle)

	ux := u[0]
	uy := u[1]
	uz := u[2]

	return fmat44.T{
		ux*ux*(1-c) + c, ux*uy*(1-c) - uz*s, ux*uz*(1-c) + uy*s, 0,
		ux*uy*(1-c) + uz*s, uy*uy*(1-c) + c, uy*uz*(1-c) - ux*s, 0,
		ux*uz*(1-c) - uy*s, uy*uz*(1-c) + ux*s, uz*uz*(1-c) + c, 0,
		0, 0, 0, 1,
	}
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
		r.LatestWorldState.BoidTarget = fvec4.UniformSphereDistribution(r.PreviousWorldState.BoidRNG, 100.0)
		r.LatestWorldState.BoidTarget[2] = 0.0
	}

	if len(r.LatestWorldState.BoidPositions) != len(r.PreviousWorldState.BoidPositions) {
		r.LatestWorldState.BoidPositions = make([]fvec4.T, len(r.PreviousWorldState.BoidPositions))
	}
	if len(r.LatestWorldState.BoidVelocities) != len(r.PreviousWorldState.BoidVelocities) {
		r.LatestWorldState.BoidVelocities = make([]fvec4.T, len(r.PreviousWorldState.BoidVelocities))
	}

	target := r.PreviousWorldState.BoidTarget

	// Boids update
	for i := 0; i < len(r.LatestWorldState.BoidPositions); i++ {
		force := fvec4.T{}

		// Every boid is attracted towards the flock target and wants to circle around it.
		offset := fvec4.SubVV(target, r.PreviousWorldState.BoidPositions[i])
		homingForce := offset
		force = fvec4.AddVV(force, homingForce)
		// circForce := fvec4.CProd3(offset, fvec4.T{0.0, 0.0, 1.0})
		// force = fvec4.AddVV(force, fvec4.Normalize(circForce))

		// Every boid gets a drag force.
		speed := r.PreviousWorldState.BoidVelocities[i].Norm()
		if speed > 1.0 {
			dragForce := fvec4.MulVS(r.PreviousWorldState.BoidVelocities[i], -1.0*0.1*speed)
			force = fvec4.AddVV(force, dragForce)
		}

		numCohesionInfluencers := 0
		averagePositionOfNeighbors := fvec4.T{}
		numAlignmentInfluencers := 0
		averageVelocityOfNeighbors := fvec4.T{}
		totalSeparationForce := fvec4.T{}
		for j := 0; j < len(r.PreviousWorldState.BoidPositions); j++ {
			// Don't influence yourself.
			if i == j {
				continue
			}

			offset := fvec4.SubVV(r.PreviousWorldState.BoidPositions[i], r.PreviousWorldState.BoidPositions[j])
			separationDistanceSquared := offset[0]*offset[0] + offset[1]*offset[1] + offset[2]*offset[2]

			if separationDistanceSquared < 9.0 {
				numCohesionInfluencers++
				averagePositionOfNeighbors = fvec4.AddVV(averagePositionOfNeighbors, r.PreviousWorldState.BoidPositions[j])
			}

			if separationDistanceSquared < 36.0 {
				numAlignmentInfluencers++
				averageVelocityOfNeighbors = fvec4.AddVV(averageVelocityOfNeighbors, r.PreviousWorldState.BoidVelocities[j])
			}

			if separationDistanceSquared < 4.0 {
				separationHeading := fvec4.Normalize(offset)
				totalSeparationForce = fvec4.AddVV(totalSeparationForce, fvec4.MulVS(separationHeading, 100.0/(1.0+separationDistanceSquared)))
			}
		}

		// Calculate cohesion
		if numCohesionInfluencers != 0 {
			averagePositionOfNeighbors = fvec4.DivVS(averagePositionOfNeighbors, float32(numCohesionInfluencers))
			cohesionOffset := fvec4.SubVV(averagePositionOfNeighbors, r.PreviousWorldState.BoidPositions[i])
			totalCohesionForce := fvec4.MulVS(cohesionOffset, 0.5)
			force = fvec4.AddVV(force, totalCohesionForce)
		}

		// Calculate alignment
		if numAlignmentInfluencers != 0 {
			averageVelocityOfNeighbors = fvec4.DivVS(averageVelocityOfNeighbors, float32(numAlignmentInfluencers))
			totalAlignmentForce := fvec4.MulVS(averageVelocityOfNeighbors, 1.0)
			force = fvec4.AddVV(force, totalAlignmentForce)
		}

		// Apply separation.
		force = fvec4.AddVV(force, totalSeparationForce)

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
	rotateX := axisAngleMat44(fvec4.T{0.0, 0.0, 1.0}, -rotateAngleX)
	rotateY := axisAngleMat44(r.LatestRenderState.Camera.Left(), rotateAngleY)
	r.LatestRenderState.Camera.ApertureToWorld = fmat44.Orthonormalize(fmat44.MulMM(fmat44.MulMM(rotateX, rotateY),
		r.LatestRenderState.Camera.ApertureToWorld))

	// TODO: Derive LatestRenderState from LatestWorldState
	// Circle above the ballistas, rather than through one of them.
	r.LatestRenderState.PointLights[0].Center = fvec4.T{
		6 * float32(math.Cos(r.LatestWorldState.SimTime)),
		6 * float32(math.Sin(r.LatestWorldState.SimTime)),
		6.0,
	}

	// Spin the middle ballista in place.
	spinningBallista := &r.LatestRenderState.MeshInstances[1]
	spinningBallista.SetModelToWorld(fmat44.MulMM(
		axisAngleMat44(fvec4.T{0, 0, 1}, float32(r.LatestWorldState.SimTime)),
		scaleMat44(4),
	))

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
		r.LatestRenderState.Tree = &bvh.Tree{}
	}
	r.LatestRenderState.Tree.Clear()
	nodes := []*bvh.Node{}
	for i := 0; i < len(r.LatestRenderState.AABBs); i++ {
		node := r.LatestRenderState.Tree.GetFreeNode()
		node.Reset(bvh.NodeTypeConcrete, r.LatestRenderState.AABBs[i].Lo, r.LatestRenderState.AABBs[i].Hi, i)
		nodes = append(nodes, node)
	}
	// Mesh instances follow the AABBs in the tree's reference space.  Each
	// instance has its own model-space tree over its triangles, so only its
	// overall bounds go in the scene tree.
	for i := range r.LatestRenderState.MeshInstances {
		inst := &r.LatestRenderState.MeshInstances[i]
		node := r.LatestRenderState.Tree.GetFreeNode()
		node.Reset(bvh.NodeTypeConcrete, inst.Lo, inst.Hi, len(r.LatestRenderState.AABBs)+i)
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

	// The camera's aperture needs to be adjusted to match the screen's ratio.
	//
	// Horizontal FOV is 80 degrees, with vertical FOV adjusted to match.
	r.LatestRenderState.Camera.Aperture[0] = 1
	r.LatestRenderState.Camera.Aperture[1] = fmath.Sin(40.0 / 180.0 * 2.0 * fmath.Pi)
	r.LatestRenderState.Camera.Aperture[2] = r.LatestRenderState.Camera.Aperture[1] / float32(rx) * float32(ry)

	if r.renderImage == nil || r.renderImage.Bounds().Dx() != rx || r.renderImage.Bounds().Dy() != ry {
		r.renderImage = ebiten.NewImage(rx, ry)
		r.buf = make([]byte, rx*ry*4)

		r.tiles = []*RenderTile{}
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
				tile := &RenderTile{
					SrcX:        tileSrcX,
					SrcY:        tileSrcY,
					LimX:        tileSrcX + tileSizeX,
					LimY:        tileSrcY + tileSizeY,
					RGBPowerBuf: make([]RGBPower, tileSizeX*tileSizeY),
					CameraRays:  make([][4]float32, tileSizeX*tileSizeY),
				}

				// Build the pinhole camera rays for this tile.
				for py := tile.SrcY; py < tile.LimY; py++ {
					ty := py - tile.SrcY
					for px := tile.SrcX; px < tile.LimX; px++ {
						tx := px - tile.SrcX

						apertureCoords := fvec4.T{
							r.LatestRenderState.Camera.Aperture[0],
							(1.0 - 2.0*float32(px)/float32(rx)) * r.LatestRenderState.Camera.Aperture[1],
							(1.0 - 2.0*float32(py)/float32(ry)) * r.LatestRenderState.Camera.Aperture[2],
						}

						tile.CameraRays[ty*tileSizeX+tx] = fvec4.Normalize(apertureCoords)
					}
				}

				r.tiles = append(r.tiles, tile)

			}
		}
	}

	// Render each tile, recording a histogram of its pixel magnitudes for
	// exposure control.
	for _, tile := range r.tiles {
		r.renderGroup.Go(func() error {
			r.LatestRenderState.RenderTile(tile, rx, ry)

			tile.MagHistogram = [exposureHistogramBins]int32{}
			for _, p := range tile.RGBPowerBuf {
				thisMag := fmath.Sqrt(p[0]*p[0] + p[1]*p[1] + p[2]*p[2])
				tile.MagHistogram[exposureHistogramBin(thisMag)]++
			}

			return nil
		})
	}
	if err := r.renderGroup.Wait(); err != nil {
		log.Printf("Error while rendering: %v", err)
	}

	// Simple automatic exposure control.  Divide all pixels by the
	// exposurePercentile pixel magnitude if it's greater than 1.0, clipping
	// the few pixels brighter than it.  Using a percentile rather than the
	// maximum keeps small hotspots (like a surface right next to a point
	// light) from darkening the whole frame.
	var histogram [exposureHistogramBins]int32
	for _, tile := range r.tiles {
		for i, n := range tile.MagHistogram {
			histogram[i] += n
		}
	}
	exposureMag := max(1, exposureHistogramPercentile(&histogram, rx*ry, exposurePercentile))
	exposureDivisor := 1 / exposureMag

	// Convert each tile to bytes directly in the output buffer.  Tiles cover
	// disjoint regions of buf, so no locking is needed.
	for _, tile := range r.tiles {
		r.renderGroup.Go(func() error {
			tileSizeX := tile.LimX - tile.SrcX
			for py := tile.SrcY; py < tile.LimY; py++ {
				ty := py - tile.SrcY
				for px := tile.SrcX; px < tile.LimX; px++ {
					tx := px - tile.SrcX
					rgbPow := tile.RGBPowerBuf[ty*tileSizeX+tx]
					out := r.buf[py*rx*4+px*4 : py*rx*4+px*4+4 : py*rx*4+px*4+4]
					for c := 0; c < 3; c++ {
						v := rgbPow[c] * exposureDivisor
						if v >= 1.0 {
							out[c] = 255
						} else {
							out[c] = byte(fmath.Floor(v * 256.0))
						}
					}
					out[3] = 255
				}
			}
			return nil
		})
	}
	if err := r.renderGroup.Wait(); err != nil {
		log.Printf("Error while converting render output: %v", err)
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

// exposurePercentile is the fraction of pixels that automatic exposure keeps
// unclipped.
const exposurePercentile = 0.999

// The exposure histogram's bins are formed from the top bits of a float32
// magnitude: its exponent plus the leading exposureHistogramMantissaBits
// mantissa bits.  That gives 2^exposureHistogramMantissaBits logarithmically
// spaced bins per power of two, without computing a logarithm per pixel.
// Magnitudes are binned over [2^exposureHistogramMinExp,
// 2^exposureHistogramMaxExp), with anything outside clamped into the first or
// last bin.
const (
	exposureHistogramMantissaBits = 4
	exposureHistogramMinExp       = -8
	exposureHistogramMaxExp       = 8
	exposureHistogramBins         = (exposureHistogramMaxExp - exposureHistogramMinExp) << exposureHistogramMantissaBits

	exposureHistogramShift    = 23 - exposureHistogramMantissaBits
	exposureHistogramFirstKey = (127 + exposureHistogramMinExp) << exposureHistogramMantissaBits
)

// exposureHistogramBin returns the histogram bin for a non-negative
// magnitude.
func exposureHistogramBin(mag float32) int {
	key := int(math.Float32bits(mag) >> exposureHistogramShift)
	return min(max(key-exposureHistogramFirstKey, 0), exposureHistogramBins-1)
}

// exposureHistogramBinUpperEdge returns the smallest magnitude past the top
// of a histogram bin.
func exposureHistogramBinUpperEdge(bin int) float32 {
	return math.Float32frombits(uint32(bin+exposureHistogramFirstKey+1) << exposureHistogramShift)
}

// exposureHistogramPercentile returns an upper bound on the magnitude at or
// below which fraction of the numPixels pixels counted in histogram fall.
func exposureHistogramPercentile(histogram *[exposureHistogramBins]int32, numPixels int, fraction float64) float32 {
	target := int(math.Ceil(fraction * float64(numPixels)))
	cumulative := 0
	for i, n := range histogram {
		cumulative += int(n)
		if cumulative >= target {
			return exposureHistogramBinUpperEdge(i)
		}
	}
	return exposureHistogramBinUpperEdge(exposureHistogramBins - 1)
}

type WorldState struct {
	SimTime float64

	CursorX, CursorY int

	CameraPosition fvec4.T
	CameraEye      fvec4.T

	BoidRNG                  *rand.Rand
	NextBoidTargetSwitchTime float64
	BoidTarget               fvec4.T
	BoidPositions            []fvec4.T
	BoidVelocities           []fvec4.T
}

type RenderState struct {
	Camera *PinholeCamera

	MaxDepth int

	Spheres           []Sphere
	AABBs             []AABB
	MeshInstances     []MeshInstance
	AmbientLights     []AmbientLight
	DirectionalLights []DirectionalLight
	PointLights       []PointLight

	Tree *bvh.Tree
}

func (r *RenderState) RenderTile(tile *RenderTile, rx, ry int) {
	tileSizeX := tile.LimX - tile.SrcX

	for py := tile.SrcY; py < tile.LimY; py++ {
		ty := py - tile.SrcY
		for px := tile.SrcX; px < tile.LimX; px++ {
			tx := px - tile.SrcX

			raySlope := fmat44.MulMV(r.Camera.ApertureToWorld, tile.CameraRays[ty*tileSizeX+tx])

			ray := Ray{
				Point: r.Camera.Center,
				Slope: raySlope,
			}
			tile.RGBPowerBuf[ty*tileSizeX+tx] = r.RenderRay(ray)
		}
	}
}

func (r *RenderState) RenderRay(ray Ray) RGBPower {
	minContact := Contact{
		Mode: ContactModeMiss,
		T:    100000000.0,
	}

	r.Tree.RayTest(ray.Point, ray.Slope, 0.0, minContact.T, func(ref int) float32 {
		var contact Contact
		if ref < len(r.AABBs) {
			contact = r.AABBs[ref].RayContact(ray, 0.0, minContact.T)
		} else {
			contact = r.MeshInstances[ref-len(r.AABBs)].RayContact(ray, 0.0, minContact.T)
		}
		if contact.Mode != ContactModeMiss {
			if contact.T > 0.0 && contact.T < minContact.T {
				minContact = contact
			}
		}
		return minContact.T
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
		dot := fvec4.IProd(contact.N, l.DirectionToSource)
		if dot > 0.0 {
			totalPower[0] += dot * l.Power[0]
			totalPower[1] += dot * l.Power[1]
			totalPower[2] += dot * l.Power[2]
		}
	}

	for _, l := range r.PointLights {
		vToLight := fvec4.SubVV(l.Center, contact.P)
		distanceToLight := vToLight.Norm()
		dirToLight := fvec4.DivVS(vToLight, distanceToLight)
		falloff := 1.0 / distanceToLight / distanceToLight

		dot := fvec4.IProd(contact.N, dirToLight)
		if dot > 0.0 {
			totalPower[0] += dot * falloff * l.Power[0]
			totalPower[1] += dot * falloff * l.Power[1]
			totalPower[2] += dot * falloff * l.Power[2]
		}
	}

	totalPower[0] *= contact.Albedo[0]
	totalPower[1] *= contact.Albedo[1]
	totalPower[2] *= contact.Albedo[2]

	return totalPower
}

type Ray struct {
	Point fvec4.T
	Slope fvec4.T
}

func (r *Ray) Eval(t float32) fvec4.T {
	return fvec4.AddVV(r.Point, fvec4.MulVS(r.Slope, t))
}

type RGBPower [3]float32

type PinholeCamera struct {
	Center          fvec4.T
	ApertureToWorld fmat44.T
	Aperture        fvec4.T
}

func (c *PinholeCamera) Eye() fvec4.T {
	return fvec4.T{
		c.ApertureToWorld[0],
		c.ApertureToWorld[4],
		c.ApertureToWorld[8],
	}
}

func (c *PinholeCamera) Left() fvec4.T {
	return fvec4.T{
		c.ApertureToWorld[1],
		c.ApertureToWorld[5],
		c.ApertureToWorld[9],
	}
}

func (c *PinholeCamera) Up() fvec4.T {
	return fvec4.T{
		c.ApertureToWorld[2],
		c.ApertureToWorld[6],
		c.ApertureToWorld[10],
	}
}

func (c *PinholeCamera) SetEye(newEye fvec4.T) {
	c.setEyeDirect(fvec4.Normalize(newEye))
	c.setUpDirect(fvec4.Normalize(fvec4.Reject(c.Eye(), c.Up())))
	c.setLeftDirect(fvec4.CProd3(c.Up(), c.Eye()))
}

func (c *PinholeCamera) SetUp(newUp fvec4.T) {
	c.setUpDirect(fvec4.Normalize(fvec4.Reject(c.Eye(), newUp)))
	c.setLeftDirect(fvec4.CProd3(c.Up(), c.Eye()))
}

func (c *PinholeCamera) setEyeDirect(newEye fvec4.T) {
	c.ApertureToWorld[0] = newEye[0]
	c.ApertureToWorld[4] = newEye[1]
	c.ApertureToWorld[8] = newEye[2]
}

func (c *PinholeCamera) setLeftDirect(newLeft fvec4.T) {
	c.ApertureToWorld[1] = newLeft[0]
	c.ApertureToWorld[5] = newLeft[1]
	c.ApertureToWorld[9] = newLeft[2]
}

func (c *PinholeCamera) setUpDirect(newUp fvec4.T) {
	c.ApertureToWorld[2] = newUp[0]
	c.ApertureToWorld[6] = newUp[1]
	c.ApertureToWorld[10] = newUp[2]
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
	P fvec4.T
	N fvec4.T

	Mtl0, Mtl1, Mtl2 float32

	// Fraction of incoming light reflected diffusely, per channel.
	Albedo RGBPower
}

type Sphere struct {
	Center fvec4.T
	Radius float32
}

func (s *Sphere) RayContact(ray Ray, queryLo, queryHi float32) Contact {
	a := ray.Slope[0]*ray.Slope[0] + ray.Slope[1]*ray.Slope[1] + ray.Slope[2]*ray.Slope[2]
	offset := fvec4.SubVV(ray.Point, s.Center)
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
			N: fvec4.Normalize(p),

			Mtl0: p[0],
			Mtl1: p[1],
			Mtl2: p[2],

			Albedo: RGBPower{1, 1, 1},
		}
	} else {
		p := ray.Eval(tMax)
		return Contact{
			Mode: ContactModeExit,

			ExitRay: ray,

			T: tMin,
			P: p,
			N: fvec4.Normalize(p),

			Mtl0: p[0],
			Mtl1: p[1],
			Mtl2: p[2],

			Albedo: RGBPower{1, 1, 1},
		}
	}
}

type AABB struct {
	Lo fvec4.T
	Hi fvec4.T
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

	minValidContact := Contact{
		Albedo: RGBPower{1, 1, 1},
	}

	// Is our contact with the X-Low plane actually on the box?
	pXLoX := b.Lo[0]
	pXLoY := ray.Point[1] + ray.Slope[1]*tXLo
	pXLoZ := ray.Point[2] + ray.Slope[2]*tXLo
	if onBox(pXLoX, pXLoY, pXLoZ, b.Lo[0], b.Hi[0], b.Lo[1], b.Hi[1], b.Lo[2], b.Hi[2]) {
		if (tXLo < minValidContact.T || minValidContact.Mode == ContactModeMiss) && queryLo <= tXLo && tXLo <= queryHi {
			minValidContact.T = tXLo
			if ray.Slope[0] >= 0 {
				minValidContact.Mode = ContactModeEnter
				minValidContact.N = fvec4.T{-1, 0, 0}

			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec4.T{1, 0, 0}
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
				minValidContact.N = fvec4.T{1, 0, 0}
			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec4.T{-1, 0, 0}
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
				minValidContact.N = fvec4.T{0, -1, 0}
			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec4.T{0, 1, 0}
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
				minValidContact.N = fvec4.T{0, 1, 0}
			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec4.T{0, -1, 0}
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
				minValidContact.N = fvec4.T{0, 0, -1}
			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec4.T{0, 0, 1}
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
				minValidContact.N = fvec4.T{0, 0, 1}
			} else {
				minValidContact.Mode = ContactModeExit
				minValidContact.N = fvec4.T{0, 0, -1}
			}
		}
	}

	if minValidContact.Mode == ContactModeMiss {
		return minValidContact
	}

	minValidContact.P = ray.Eval(minValidContact.T)

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
	DirectionToSource fvec4.T
	Power             RGBPower
}

type PointLight struct {
	Center fvec4.T
	Power  RGBPower
}
