package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
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
	initialWorldState.Viewport = Viewport{
		Center:             vec2.T{0.0, 0.0},
		Angle:              0.0,
		PixelsPerWorldUnit: 50.0,
	}

	r := &Renderer{
		prevState: initialWorldState,
		nextState: initialWorldState.Clone(),
	}

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("Asteroids")
	if err := ebiten.RunGame(r); err != nil {
		return fmt.Errorf("while running game: %w", err)
	}

	return nil
}

type Renderer struct {
	prevState, nextState *WorldState
}

var _ ebiten.Game = (*Renderer)(nil)

func (r *Renderer) Update() error {
	r.nextState, r.prevState = r.prevState, r.nextState

	r.nextState.CurStep = r.prevState.CurStep + 1
	r.nextState.CurSimTime = r.prevState.CurSimTime + 1000000000/60
	r.nextState.CurRealTime = time.Now()

	deltaSec := 1.0 / 60.0

	shipThrust := 0.0
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		shipThrust += 1.0
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		shipThrust -= 0.5
	}
	shipThrustVec := mat22.MulMV(r.prevState.ShipOrientation, vec2.T{0.0, shipThrust})
	r.nextState.ShipVelocity[0] = r.prevState.ShipVelocity[0] + shipThrustVec[0]*deltaSec
	r.nextState.ShipVelocity[1] = r.prevState.ShipVelocity[1] + shipThrustVec[1]*deltaSec
	r.nextState.ShipPosition[0] = r.prevState.ShipPosition[0] + r.prevState.ShipVelocity[0]*deltaSec
	r.nextState.ShipPosition[1] = r.prevState.ShipPosition[1] + r.prevState.ShipVelocity[1]*deltaSec

	shipRotate := 0.0
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		shipRotate += 1
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		shipRotate -= 1
	}
	shipRotateStep := mat22.Rotate(deltaSec * shipRotate)
	r.nextState.ShipOrientation = mat22.MulMM(shipRotateStep, r.prevState.ShipOrientation)

	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	viewport := r.nextState.Viewport
	viewport.ScreenWidthPixels, viewport.ScreenHeightPixels = ebiten.WindowSize()
	worldToScreen := viewport.WorldToScreenMatrix()

	shipModelToWorld := mat33.T{}
	shipModelToWorld[0*3+0] = r.nextState.ShipOrientation[0*2+0] * 1.0 / 64
	shipModelToWorld[0*3+1] = r.nextState.ShipOrientation[0*2+1] * 1.0 / 64
	shipModelToWorld[0*3+2] = r.nextState.ShipPosition[0]
	shipModelToWorld[1*3+0] = r.nextState.ShipOrientation[1*2+0] * 1.0 / 64
	shipModelToWorld[1*3+1] = r.nextState.ShipOrientation[1*2+1] * 1.0 / 64
	shipModelToWorld[1*3+2] = r.nextState.ShipPosition[1]
	shipModelToWorld[2*3+0] = 0
	shipModelToWorld[2*3+1] = 0
	shipModelToWorld[2*3+2] = 1
	simplespacespritesheet.Sheet.RenderAtWorldPosition(screen, simplespacespritesheet.SHIP_B, worldToScreen, shipModelToWorld)

	shipScreenCenter := mat33.MulMV2(worldToScreen, r.nextState.ShipPosition)
	vector.DrawFilledCircle(screen, float32(shipScreenCenter[0]), float32(shipScreenCenter[1]), 4, color.RGBA{0, 255, 0, 255}, true)
}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

type WorldState struct {
	CurStep     int64
	CurSimTime  int64
	CurRealTime time.Time

	ShipPosition    vec2.T
	ShipVelocity    vec2.T
	ShipOrientation mat22.T

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
	out.Viewport = w.Viewport
	return out
}

type Viewport struct {
	Center             vec2.T
	Angle              float64
	PixelsPerWorldUnit float64
	ScreenWidthPixels  int
	ScreenHeightPixels int
}

func (v *Viewport) WorldToScreenMatrix() mat33.T {
	worldToViewport := mat33.Translate2(vec2.T{-v.Center[0], -v.Center[1]})
	worldToViewport = mat33.MulMM(mat33.Rotate2(v.Angle), worldToViewport)
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
