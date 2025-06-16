package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand/v2"
	"shellworld/embeddedassets/simplespacespritesheet"
	"shellworld/lib/vmath/mat22"
	"shellworld/lib/vmath/mat33"
	"shellworld/lib/vmath/vec2"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func main() {
	flag.Parse()

	if err := do(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func do() error {
	if err := simplespacespritesheet.Load(); err != nil {
		return fmt.Errorf("while loading simplespacespritesheet: %w", err)
	}

	initialWorldState := &WorldState{}
	initialWorldState.ShipPosition = vec2.T{}
	initialWorldState.ShipOrientation = mat22.Rotate(0)

	rng := rand.New(rand.NewChaCha8([32]byte{}))
	for i := 0; i < 30; i++ {
		initialWorldState.AsteroidPositions = append(initialWorldState.AsteroidPositions, vec2.UniformDiskDistribution(rng, 10))
	}

	dustRNG := rand.New(rand.NewChaCha8([32]byte{}))
	initialWorldState.DustPositions = make([]vec2.T, 1000)
	for i := 0; i < 1000; i++ {
		initialWorldState.DustPositions[i] = vec2.UniformDiskDistribution(rng, 100)
	}

	initialWorldState.Viewport = Viewport{
		Center:             vec2.T{0.0, 0.0},
		Orientation:        mat22.Identity(),
		PixelsPerWorldUnit: 50.0,
	}

	r := &Renderer{
		World: &World{
			PrevState: initialWorldState,
			NextState: initialWorldState.Clone(),

			DustRNG: dustRNG,
		},
	}

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("Asteroids")
	if err := ebiten.RunGame(r); err != nil {
		return fmt.Errorf("while running game: %w", err)
	}

	return nil
}

type Renderer struct {
	World *World
}

var _ ebiten.Game = (*Renderer)(nil)

func (r *Renderer) Update() error {
	r.World.NextState, r.World.PrevState = r.World.PrevState, r.World.NextState

	r.World.NextState.CurStep = r.World.PrevState.CurStep + 1
	r.World.NextState.CurSimTime = r.World.PrevState.CurSimTime + 1000000000/60
	r.World.NextState.CurRealTime = time.Now()

	deltaSec := 1.0 / 60.0

	shipThrust := 0.0
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		shipThrust += 5.0
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		shipThrust -= 2.5
	}
	shipThrustVec := mat22.MulMV(r.World.PrevState.ShipOrientation, vec2.T{0.0, shipThrust})
	r.World.NextState.ShipVelocity[0] = r.World.PrevState.ShipVelocity[0] + shipThrustVec[0]*deltaSec
	r.World.NextState.ShipVelocity[1] = r.World.PrevState.ShipVelocity[1] + shipThrustVec[1]*deltaSec
	r.World.NextState.ShipPosition[0] = r.World.PrevState.ShipPosition[0] + r.World.PrevState.ShipVelocity[0]*deltaSec
	r.World.NextState.ShipPosition[1] = r.World.PrevState.ShipPosition[1] + r.World.PrevState.ShipVelocity[1]*deltaSec

	shipRotate := 0.0
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		shipRotate += 2 * math.Pi * 0.7
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		shipRotate -= 2 * math.Pi * 0.7
	}
	shipRotateStep := mat22.Rotate(deltaSec * shipRotate)
	r.World.NextState.ShipOrientation = mat22.MulMM(shipRotateStep, r.World.PrevState.ShipOrientation)

	// Viewport follows the ship
	r.World.NextState.Viewport.Center = r.World.NextState.ShipPosition
	r.World.NextState.Viewport.Orientation = r.World.NextState.ShipOrientation

	for i := range r.World.NextState.AsteroidPositions {
		r.World.NextState.AsteroidPositions[i] = r.World.PrevState.AsteroidPositions[i]
	}

	// Dust that's too far from the ship is randomly placed closer to the ship
	for i := range r.World.NextState.DustPositions {
		v := vec2.Sub(r.World.NextState.ShipPosition, r.World.PrevState.DustPositions[i])
		if v[0]*v[0]+v[1]*v[1] > 100.0*100.0 {
			r.World.NextState.DustPositions[i] = vec2.UniformDiskDistribution(r.World.DustRNG, 1000.0)
		} else {
			r.World.NextState.DustPositions[i] = r.World.PrevState.DustPositions[i]
		}
	}

	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	viewport := r.World.NextState.Viewport
	viewport.ScreenWidthPixels, viewport.ScreenHeightPixels = ebiten.WindowSize()
	worldToScreen := viewport.WorldToScreenMatrix()

	shipModelToWorld := mat33.T{}
	shipModelToWorld[0*3+0] = r.World.NextState.ShipOrientation[0*2+0] * 1.0 / 64
	shipModelToWorld[0*3+1] = r.World.NextState.ShipOrientation[0*2+1] * 1.0 / 64
	shipModelToWorld[0*3+2] = r.World.NextState.ShipPosition[0]
	shipModelToWorld[1*3+0] = r.World.NextState.ShipOrientation[1*2+0] * 1.0 / 64
	shipModelToWorld[1*3+1] = r.World.NextState.ShipOrientation[1*2+1] * 1.0 / 64
	shipModelToWorld[1*3+2] = r.World.NextState.ShipPosition[1]
	shipModelToWorld[2*3+0] = 0
	shipModelToWorld[2*3+1] = 0
	shipModelToWorld[2*3+2] = 1
	simplespacespritesheet.Sheet.RenderAtWorldPosition(screen, simplespacespritesheet.SHIP_B, worldToScreen, shipModelToWorld)

	shipScreenCenter := mat33.MulMV2(worldToScreen, r.World.NextState.ShipPosition)
	vector.DrawFilledCircle(screen, float32(shipScreenCenter[0]), float32(shipScreenCenter[1]), 4, color.RGBA{0, 255, 0, 255}, true)

	// Render asteroids
	for i := range r.World.NextState.AsteroidPositions {
		modelToWorld := mat33.T{
			1.0 / 64, 0, r.World.NextState.AsteroidPositions[i][0],
			0, 1.0 / 64, r.World.NextState.AsteroidPositions[i][1],
			0, 0, 1,
		}
		simplespacespritesheet.Sheet.RenderAtWorldPosition(screen, simplespacespritesheet.METEOR_DETAILEDLARGE, worldToScreen, modelToWorld)
	}

	// Render dust
	for i := range r.World.NextState.DustPositions {
		x := r.World.NextState.DustPositions[i][0] - r.World.NextState.ShipPosition[0]
		y := r.World.NextState.DustPositions[i][1] - r.World.NextState.ShipPosition[1]
		if x*x+y*y >= 100*100 {
			continue
		}

		modelToWorld := mat33.T{
			1.0 / 128, 0, r.World.NextState.DustPositions[i][0],
			0, 1.0 / 128, r.World.NextState.DustPositions[i][1],
			0, 0, 1,
		}
		simplespacespritesheet.Sheet.RenderAtWorldPosition(screen, simplespacespritesheet.STAR_TINY, worldToScreen, modelToWorld)
	}

}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

type World struct {
	PrevState *WorldState
	NextState *WorldState

	DustRNG *rand.Rand
}

type WorldState struct {
	CurStep     int64
	CurSimTime  int64
	CurRealTime time.Time

	ShipPosition    vec2.T
	ShipVelocity    vec2.T
	ShipOrientation mat22.T

	AsteroidPositions []vec2.T

	DustPositions []vec2.T

	Viewport Viewport
}

func (w *WorldState) Clone() *WorldState {
	out := &WorldState{}
	out.CurStep = w.CurStep
	out.CurSimTime = w.CurSimTime
	out.CurRealTime = w.CurRealTime
	out.ShipPosition = w.ShipPosition
	out.ShipVelocity = w.ShipVelocity
	out.ShipOrientation = w.ShipOrientation

	out.AsteroidPositions = make([]vec2.T, len(w.AsteroidPositions))
	for i := range w.AsteroidPositions {
		out.AsteroidPositions[i] = w.AsteroidPositions[i]
	}

	out.DustPositions = make([]vec2.T, len(w.DustPositions))
	for i := range w.DustPositions {
		out.DustPositions[i] = w.DustPositions[i]
	}

	out.Viewport = w.Viewport
	return out
}

type Viewport struct {
	Center             vec2.T
	Orientation        mat22.T
	PixelsPerWorldUnit float64
	ScreenWidthPixels  int
	ScreenHeightPixels int
}

func (v *Viewport) WorldToScreenMatrix() mat33.T {
	worldToViewport := mat33.Inverse(mat33.Transform2(1.0, v.Orientation, v.Center))
	worldToViewport = mat33.MulMM(mat33.Scale(v.PixelsPerWorldUnit), worldToViewport)

	vpToScreen := mat33.Scale2(1.0, -1.0)
	vpToScreen = mat33.MulMM(mat33.Translate2(vec2.T{float64(v.ScreenWidthPixels) / 2.0, float64(v.ScreenHeightPixels) / 2.0}), vpToScreen)

	worldToScreen := mat33.MulMM(vpToScreen, worldToViewport)
	return worldToScreen
}

func (v *Viewport) WorldPositionToScreenPosition(worldPos vec2.T) vec2.T {
	return mat33.MulMV2(v.WorldToScreenMatrix(), worldPos)
}

func (v *Viewport) ScreenPositionToWorldPosition(screenPos vec2.T) vec2.T {
	screenToWorld := mat33.Inverse(v.WorldToScreenMatrix())
	worldPos := mat33.MulMV2(screenToWorld, screenPos)
	return worldPos
}
