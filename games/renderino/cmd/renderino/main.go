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
	"math/bits"
	"math/rand/v2"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime/pprof"
	"time"

	"github.com/ahmedtd/gogames/embeddedassets/departuremonofont"
	"github.com/ahmedtd/gogames/embeddedassets/kenneycastlekit"
	"github.com/ahmedtd/gogames/embeddedassets/kenneyprototypekit"
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

	// Number of primary rays traced per pixel, averaged to give the pixel's
	// value.  Always at least 1.
	curSupersampling int

	renderGroup *errgroup.Group

	// Model drawn for each boid.  Boid instances follow the
	// numStaticMeshInstances static ones in LatestRenderState.MeshInstances.
	boidModel              *MeshModel
	numStaticMeshInstances int

	TileSize int
	tiles    []*RenderTile

	// Resolution and supersampling that tiles and buf were allocated for.
	frameRX, frameRY   int
	frameSupersampling int

	// Work done raytracing the most recent frame, summed over all tiles.
	FrameStats RayStats

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

	// Work done raytracing this tile, from the most recent render.  Each
	// render thread counts into its own tile, so no atomics are needed.
	Stats RayStats

	// Scratch space for BVH traversals while raytracing this tile.  As with
	// Stats, only the thread rendering this tile uses it.
	BVHStack bvh.Stack

	// Size (LimX-SrcX)*(LimY-SrcY)
	IndexBuf [][4]float32

	// Size (LimX-SrcX)*(LimY-SrcY)*Supersampling.  The rays for each pixel
	// are contiguous.
	CameraRays    [][4]float32
	Supersampling int

	// The nearest surface under each camera ray, when primary rays are
	// replaced by rasterization.  Laid out like CameraRays.
	GBuf *GBuffer
}

var _ ebiten.Game = &Renderer{}

func NewRenderer(numRenderThreads int) (*Renderer, error) {
	r, err := newHeadlessRenderer(numRenderThreads)
	if err != nil {
		return nil, err
	}

	uiFaceSource, err := text.NewGoTextFaceSource(bytes.NewReader(departuremonofont.DepartureMonoOTF))
	if err != nil {
		return nil, fmt.Errorf("while loading Departure Mono font: %w", err)
	}
	r.uiFaceSource = uiFaceSource
	r.uiNormalFace = &text.GoTextFace{
		Source: uiFaceSource,
		Size:   24,
	}

	cursorX, cursorY := ebiten.CursorPosition()
	r.PreviousWorldState.CursorX, r.PreviousWorldState.CursorY = cursorX, cursorY

	return r, nil
}

// newHeadlessRenderer builds a Renderer with its scene and initial world
// state, but without anything that needs a running ebiten game (UI fonts,
// cursor position), so that benchmarks can render frames off-screen.
func newHeadlessRenderer(numRenderThreads int) (*Renderer, error) {
	camera := &PinholeCamera{
		Center:          fvec4.T{-10, 0, 10},
		ApertureToWorld: fmat44.Identity(),
		Aperture:        fvec4.T{0.02, 0.018, 0.012},
	}

	renderGroup := &errgroup.Group{}
	renderGroup.SetLimit(numRenderThreads)

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

	floorMesh, err := kenneyprototypekit.Load("floor-square", true)
	if err != nil {
		return nil, fmt.Errorf("while loading floor mesh: %w", err)
	}
	floorModel := NewMeshModel(floorMesh)

	dogMesh, err := kenneyprototypekit.Load("animal-dog", true)
	if err != nil {
		return nil, fmt.Errorf("while loading animal-dog mesh: %w", err)
	}
	dogModel := NewMeshModel(dogMesh)

	// Lay a 10x10 grid of floor tiles centered on the origin, with their tops
	// at z = 0.  Tiles are scaled to match the ballistas.
	const floorTileScale = 4
	floorTileSize := floorTileScale * (floorModel.Hi[0] - floorModel.Lo[0])
	floorTileZ := -floorTileScale * floorModel.Hi[2]
	var floorTiles []MeshInstance
	for i := 0; i < 40; i++ {
		for j := 0; j < 40; j++ {
			modelToWorld := fmat44.MulMM(
				translateMat44(fvec4.T{
					(float32(i) - 19.5) * floorTileSize,
					(float32(j) - 19.5) * floorTileSize,
					floorTileZ,
				}),
				scaleMat44(floorTileScale),
			)
			floorTiles = append(floorTiles, NewMeshInstance(floorModel, modelToWorld))
		}
	}

	staticMeshInstances := append(ballistas, floorTiles...)

	staticScene := &RenderState{
		Camera: camera,

		RasterPrimary: true,
		Shadows:       true,
		CullBackFaces: true,

		MeshInstances: staticMeshInstances,

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

	rng := rand.New(rand.NewPCG(1, 2))

	// Initialize boids.  Each boid is one mesh in the scene, so this sets
	// how hard the renderer is taxed; the O(n^2) boid update is serial, so keep
	// it from dominating frame time.
	const numBoids = 40
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
		curSupersampling: 1,

		renderGroup: renderGroup,

		boidModel:              dogModel,
		numStaticMeshInstances: len(staticMeshInstances),

		TileSize: 64,

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
	if inpututil.IsKeyJustPressed(ebiten.Key8) {
		r.curSupersampling = max(r.curSupersampling-1, 1)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key9) {
		r.curSupersampling++
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		r.LatestRenderState.RasterPrimary = !r.LatestRenderState.RasterPrimary
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyO) {
		r.LatestRenderState.Shadows = !r.LatestRenderState.Shadows
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

	r.updateRenderState(r.LatestWorldState)

	r.PreviousWorldState, r.LatestWorldState = r.LatestWorldState, r.PreviousWorldState

	return nil
}

// updateRenderState derives the parts of LatestRenderState that follow the
// simulation (lights, animated meshes, boids, and the scene BVH) from
// world state ws.  The camera is driven by input directly, in Update.
func (r *Renderer) updateRenderState(ws *WorldState) {
	// TODO: Derive LatestRenderState from LatestWorldState
	// Circle above the ballistas, rather than through one of them.
	r.LatestRenderState.PointLights[0].Center = fvec4.T{
		6 * float32(math.Cos(ws.SimTime)),
		6 * float32(math.Sin(ws.SimTime)),
		6.0,
	}

	// Spin the middle ballista in place.
	spinningBallista := &r.LatestRenderState.MeshInstances[1]
	spinningBallista.SetModelToWorld(fmat44.MulMM(
		axisAngleMat44(fvec4.T{0, 0, 1}, float32(ws.SimTime)),
		scaleMat44(4),
	))

	// Each boid is a dog at the boid's position, turned to face along its
	// velocity.  The dog model faces model-space -X, so it's turned an extra
	// half turn.
	numInstances := r.numStaticMeshInstances + len(ws.BoidPositions)
	for len(r.LatestRenderState.MeshInstances) < numInstances {
		r.LatestRenderState.MeshInstances = append(r.LatestRenderState.MeshInstances, MeshInstance{Model: r.boidModel})
	}
	r.LatestRenderState.MeshInstances = r.LatestRenderState.MeshInstances[:numInstances]
	for i, pos := range ws.BoidPositions {
		vel := ws.BoidVelocities[i]
		heading := fmath.Atan2(vel[1], vel[0]) + fmath.Pi
		r.LatestRenderState.MeshInstances[r.numStaticMeshInstances+i].SetModelToWorld(fmat44.MulMM(
			translateMat44(pos),
			axisAngleMat44(fvec4.T{0, 0, 1}, heading),
		))
	}

	// Build tree.  Each mesh instance has its own model-space tree over its
	// triangles, so only its overall bounds go in the scene tree.
	if r.LatestRenderState.Tree == nil {
		r.LatestRenderState.Tree = &bvh.Tree{}
	}
	r.LatestRenderState.Tree.Clear()
	nodes := make([]*bvh.Node, 0, len(r.LatestRenderState.MeshInstances))
	for i := range r.LatestRenderState.MeshInstances {
		inst := &r.LatestRenderState.MeshInstances[i]
		node := r.LatestRenderState.Tree.GetFreeNode()
		node.Reset(bvh.NodeTypeConcrete, inst.Lo, inst.Hi, i)
		nodes = append(nodes, node)
	}
	r.LatestRenderState.Tree.BuildFromElements(nodes)
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

	r.renderFrame(rx, ry)

	if r.renderImage == nil || r.renderImage.Bounds().Dx() != rx || r.renderImage.Bounds().Dy() != ry {
		r.renderImage = ebiten.NewImage(rx, ry)
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
	infoText := fmt.Sprintf("fps=%f\ncurUpscaleFactor=%f\ncurSupersampling=%d\nrasterPrimary=%v\nshadows=%v\ncursor=%d,%d", 1.0/drawDelta.Seconds(), r.curUpscaleFactor, r.curSupersampling, r.LatestRenderState.RasterPrimary, r.LatestRenderState.Shadows, cursorX, cursorY)
	w, h := text.Measure(infoText, r.uiNormalFace, r.uiNormalFace.Size*1.5)
	vector.DrawFilledRect(screen, x, y, float32(w), float32(h), color.RGBA{0x80, 0x80, 0x80, 0xff}, false)
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.LineSpacing = r.uiNormalFace.Size * 1.5
	text.Draw(screen, infoText, r.uiNormalFace, op)

	r.lastDrawStart = drawStart
}

// renderFrame raytraces LatestRenderState at rx by ry pixels, leaving the
// exposure-corrected RGBA result in r.buf.  It doesn't touch ebiten, so it
// can run headless.
func (r *Renderer) renderFrame(rx, ry int) {
	r.LatestRenderState.Camera.FitAperture(rx, ry)
	if r.LatestRenderState.RasterPrimary {
		r.LatestRenderState.prepareRaster(rx, ry)
	}

	samples := max(r.curSupersampling, 1)

	if r.tiles == nil || r.frameRX != rx || r.frameRY != ry || r.frameSupersampling != samples {
		r.frameRX, r.frameRY = rx, ry
		r.frameSupersampling = samples
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
					SrcX:          tileSrcX,
					SrcY:          tileSrcY,
					LimX:          tileSrcX + tileSizeX,
					LimY:          tileSrcY + tileSizeY,
					RGBPowerBuf:   make([]RGBPower, tileSizeX*tileSizeY),
					CameraRays:    make([][4]float32, tileSizeX*tileSizeY*samples),
					Supersampling: samples,
					GBuf:          NewGBuffer(tileSrcX, tileSrcY, tileSrcX+tileSizeX, tileSrcY+tileSizeY, samples),
				}

				// Build the pinhole camera rays for this tile, samples per
				// pixel, spread across the pixel by supersampleOffset.
				for py := tile.SrcY; py < tile.LimY; py++ {
					ty := py - tile.SrcY
					for px := tile.SrcX; px < tile.LimX; px++ {
						tx := px - tile.SrcX
						for s := 0; s < samples; s++ {
							ox, oy := supersampleOffset(s, samples)
							tile.CameraRays[(ty*tileSizeX+tx)*samples+s] = r.LatestRenderState.Camera.ApertureRay(float32(px)+ox, float32(py)+oy, rx, ry)
						}
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

	r.FrameStats = RayStats{}
	for _, tile := range r.tiles {
		r.FrameStats.Add(&tile.Stats)
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
}

// supersampleOffset returns where, within a pixel, sample i of n lies, as
// offsets in [0, 1) from the pixel's top-left corner.  The samples form a
// Hammersley set (shifted so a lone sample sits at the pixel center), which
// spreads them evenly over the pixel in both directions for any n.  The
// pattern is fixed, so there's no frame-to-frame noise.
func supersampleOffset(i, n int) (float32, float32) {
	halfStep := 0.5 / float32(n)
	return float32(i)/float32(n) + halfStep, radicalInverse2(uint32(i)) + halfStep
}

// radicalInverse2 mirrors the base-2 digits of i about the binary point,
// giving a value in [0, 1).
func radicalInverse2(i uint32) float32 {
	return float32(bits.Reverse32(i)) * 0x1p-32
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

	MeshInstances     []MeshInstance
	AmbientLights     []AmbientLight
	DirectionalLights []DirectionalLight
	PointLights       []PointLight

	Tree *bvh.Tree

	// If set, the first contact of each camera ray is found by rasterizing
	// the scene rather than raytracing it.
	RasterPrimary bool

	// If set, rasterization skips triangles facing away from the camera.
	CullBackFaces bool

	// If set, directional and point lights only light surfaces that can see
	// them, as tested with shadow rays.  Otherwise, they light every surface
	// facing them.
	Shadows bool

	// Rasterization setup for the current frame; see prepareRaster.
	projection RasterProjection

	// Per mesh instance, its model-to-clip transform.
	instanceToClip []fmat44.T
}

// prepareRaster sets up rasterizing the scene to an rx by ry image, with the
// camera's aperture already fitted to it.
func (r *RenderState) prepareRaster(rx, ry int) {
	r.projection = NewRasterProjection(r.Camera, rx, ry)
	r.instanceToClip = r.instanceToClip[:0]
	for i := range r.MeshInstances {
		r.instanceToClip = append(r.instanceToClip, fmat44.MulMM(r.projection.WorldToClip, r.MeshInstances[i].ModelToWorld()))
	}
}

// rasterizeTile fills tile.GBuf with the nearest surface under each of the
// tile's camera rays.  Only the triangles in BVH nodes overlapping the tile's
// frustum are rasterized.
func (r *RenderState) rasterizeTile(tile *RenderTile) {
	gb := tile.GBuf
	gb.Clear()
	stack := &tile.BVHStack
	bvhStats := &tile.Stats.BVH

	planes := rasterTilePlanes(&r.projection.WorldToClip, gb.SrcX, gb.SrcY, gb.LimX, gb.LimY)
	r.Tree.FrustumTest(planes[:], stack, bvhStats, func(instRef int) {
		inst := &r.MeshInstances[instRef]
		model := inst.Model
		toClip := &r.instanceToClip[instRef]

		// The same tile bounds, as planes in the instance's model space.
		modelPlanes := rasterTilePlanes(toClip, gb.SrcX, gb.SrcY, gb.LimX, gb.LimY)
		model.Tree.FrustumTest(modelPlanes[:], stack, bvhStats, func(packetRef int) {
			p := &model.Packets[packetRef]
			for _, triRef := range p.Tris[:p.N] {
				tv := &model.TriVerts[triRef]
				clip := [3]fvec4.T{
					fmat44.MulMV(*toClip, model.Vertices[tv[0]]),
					fmat44.MulMV(*toClip, model.Vertices[tv[1]]),
					fmat44.MulMV(*toClip, model.Vertices[tv[2]]),
				}
				tri := &model.Tris[triRef]
				attrs := RasterAttrs{
					Normals: [3]fvec4.T{
						inst.normalToWorld(tri.Ns[0]),
						inst.normalToWorld(tri.Ns[1]),
						inst.normalToWorld(tri.Ns[2]),
					},
					GeomNormal: inst.normalToWorld(fvec4.CProd3(tri.E1, tri.E2)),
					UVs:        tri.UVs,
					Diffuse:    model.Diffuse,
				}
				gb.RasterizeTri(clip, &attrs, r.CullBackFaces, VisHit{Inst: int32(instRef), Tri: triRef})
			}
		})
	})
	tile.Stats.Raster = gb.Stats
}

func (r *RenderState) RenderTile(tile *RenderTile, rx, ry int) {
	tileSizeX := tile.LimX - tile.SrcX
	samples := tile.Supersampling
	sampleWeight := 1 / float32(samples)
	tile.Stats = RayStats{}

	if r.RasterPrimary {
		r.rasterizeTile(tile)
	}

	for py := tile.SrcY; py < tile.LimY; py++ {
		ty := py - tile.SrcY
		for px := tile.SrcX; px < tile.LimX; px++ {
			tx := px - tile.SrcX
			pixelIndex := ty*tileSizeX + tx

			var sum RGBPower
			for s, cameraRay := range tile.CameraRays[pixelIndex*samples : (pixelIndex+1)*samples] {
				ray := Ray{
					Point: r.Camera.Center,
					Slope: fmat44.MulMV(r.Camera.ApertureToWorld, cameraRay),
				}
				var p RGBPower
				if r.RasterPrimary {
					p = r.RenderGBufferSample(ray, cameraRay[0], tile.GBuf, pixelIndex*samples+s, &tile.BVHStack, &tile.Stats)
				} else {
					p = r.RenderRay(ray, &tile.BVHStack, &tile.Stats)
				}
				sum[0] += p[0]
				sum[1] += p[1]
				sum[2] += p[2]
			}
			tile.RGBPowerBuf[pixelIndex] = RGBPower{sum[0] * sampleWeight, sum[1] * sampleWeight, sum[2] * sampleWeight}
		}
	}
}

// RayStats counts raytracing work, for measuring how much work the renderer
// does per ray independently of how fast it does it.  It's plain counters, so
// each render thread should count into its own RayStats and merge them
// afterwards with Add.
type RayStats struct {
	BVH bvh.Stats // Summed over every tree: scene-level and per-mesh.

	TriTests int64 // Ray-triangle intersection tests.

	Rays    int64 // Primary rays traced.
	RayHits int64 // Primary rays that hit something.

	ShadowRays int64 // Shadow rays traced towards lights.

	Raster RasterStats // Rasterization in place of primary rays.
}

// Add adds o's counts into s.
func (s *RayStats) Add(o *RayStats) {
	s.BVH.Add(&o.BVH)
	s.TriTests += o.TriTests
	s.Rays += o.Rays
	s.RayHits += o.RayHits
	s.ShadowRays += o.ShadowRays
	s.Raster.Add(&o.Raster)
}

// RenderRay traces a primary ray, using stack for BVH traversals and adding
// the work done into stats.
func (r *RenderState) RenderRay(ray Ray, stack *bvh.Stack, stats *RayStats) RGBPower {
	minContact := Contact{
		Mode: ContactModeMiss,
		T:    100000000.0,
	}

	r.Tree.RayTest(ray.Point, ray.Slope, 0.0, minContact.T, stack, &stats.BVH, func(ref int) float32 {
		contact := r.MeshInstances[ref].RayContact(ray, 0.0, minContact.T, stack, stats)
		if contact.Mode != ContactModeMiss {
			if contact.T > 0.0 && contact.T < minContact.T {
				minContact = contact
			}
		}
		return minContact.T
	})

	stats.Rays++
	if minContact.Mode == ContactModeMiss {
		return missPower(ray)
	}

	stats.RayHits++
	power := r.matteShade(&minContact, stack, stats)
	return power
}

// RenderGBufferSample is RenderRay for a primary ray whose first contact was
// found by rasterization, as sample i of gb.  eyeSlope is the component of
// the ray's slope along the camera's eye axis.
func (r *RenderState) RenderGBufferSample(ray Ray, eyeSlope float32, gb *GBuffer, i int, stack *bvh.Stack, stats *RayStats) RGBPower {
	stats.Rays++
	hit := gb.Hits[i]
	if hit.Inst < 0 {
		return missPower(ray)
	}
	stats.RayHits++

	// The contact is at eye-axis depth W = 1/InvDepth.
	t := 1 / (gb.InvDepth[i] * eyeSlope)
	n := gb.Normals[i]
	contact := Contact{
		Mode:    ContactModeEnter,
		ExitRay: ray,
		T:       t,
		P:       ray.Eval(t),
		N:       fvec4.Normalize(fvec4.T{n[0], n[1], n[2], 0}),
		Albedo:  gb.Albedos[i],
	}
	contact.P[3] = 0
	if hit.Back {
		contact.Mode = ContactModeExit
	}
	return r.matteShade(&contact, stack, stats)
}

// missPower is the light arriving along a ray that leaves the scene: a simple
// horizon at infinity, for orientation.
func missPower(ray Ray) RGBPower {
	if ray.Slope[2] < 0.0 {
		return RGBPower{0.0, 0.0, 0.0}
	}
	return RGBPower{0.0, 0.1, 0.5}
}

// shadowBias is how far shadow rays start off a surface, along its normal,
// so that they don't immediately hit the surface they're leaving due to
// floating-point error.
const shadowBias = 1e-3

// shadowCutoffPower is the light power below which matteShade skips a point
// light's shadow ray.  Exposure only ever divides pixel power (by at least
// 1), and the output has 256 levels per unit power, so skipping the shadow
// ray can brighten a pixel that should be in shadow by at most this many
// levels.  Four levels is barely visible, and in wide views it saves a large
// share of the point light's shadow rays.
const shadowCutoffPower = 4.0 / 256

// Occluded reports whether anything in the scene blocks ray for t in
// (tLo, tHi).  It stops at the first blocker found, rather than searching for
// the nearest.
func (r *RenderState) Occluded(ray Ray, tLo, tHi float32, stack *bvh.Stack, stats *RayStats) bool {
	stats.ShadowRays++
	occluded := false
	r.Tree.RayTestUnordered(ray.Point, ray.Slope, tLo, tHi, stack, &stats.BVH, func(ref int) float32 {
		if r.MeshInstances[ref].RayOccluded(ray, tLo, tHi, stack, stats) {
			occluded = true
			// A bound below tLo prunes the rest of the traversal.
			return float32(math.Inf(-1))
		}
		return tHi
	})
	return occluded
}

// matteShade computes the diffuse light leaving contact.  With Shadows set,
// each directional and point light only contributes if a shadow ray from the
// contact can reach it.
func (r *RenderState) matteShade(contact *Contact, stack *bvh.Stack, stats *RayStats) RGBPower {
	totalPower := RGBPower{}

	// Shadow rays leave from just off the surface, on the side facing the
	// viewer.  Lights are only considered when they're on that side too.
	shadowOrigin := fvec4.AddVV(contact.P, fvec4.MulVS(contact.N, shadowBias))

	for _, l := range r.AmbientLights {
		totalPower[0] += l.Power[0]
		totalPower[1] += l.Power[1]
		totalPower[2] += l.Power[2]
	}

	for _, l := range r.DirectionalLights {
		dot := fvec4.IProd(contact.N, l.DirectionToSource)
		// Directional lights are infinitely far away, so the shadow ray
		// only reaches the light if it leaves the scene without a contact.
		if dot <= 0.0 {
			continue
		}
		shadowRay := Ray{Point: shadowOrigin, Slope: l.DirectionToSource}
		if !r.Shadows || !r.Occluded(shadowRay, 0, float32(math.Inf(1)), stack, stats) {
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
		if dot <= 0.0 {
			continue
		}

		// A light too dim here to visibly change the pixel isn't worth a
		// shadow ray; its contribution is added unshadowed.
		lit := !r.Shadows || dot*falloff*max(l.Power[0], l.Power[1], l.Power[2]) < shadowCutoffPower
		if !lit {
			// With the unnormalized slope, the light is at t = 1, so
			// anything with t < 1 is between the contact and the light.
			shadowRay := Ray{Point: shadowOrigin, Slope: fvec4.SubVV(l.Center, shadowOrigin)}
			lit = !r.Occluded(shadowRay, 0, 1, stack, stats)
		}
		if lit {
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
func (c *PinholeCamera) ApertureRay(x, y float32, rx, ry int) fvec4.T {
	return fvec4.Normalize(fvec4.T{
		c.Aperture[0],
		(1.0 - 2.0*x/float32(rx)) * c.Aperture[1],
		(1.0 - 2.0*y/float32(ry)) * c.Aperture[2],
	})
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
