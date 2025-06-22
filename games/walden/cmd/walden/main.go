package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"log"
	"math/rand/v2"
	"time"

	"github.com/ahmedtd/gogames/embeddedassets/kenneyrtsmedievalspritesheet"
	"github.com/ahmedtd/gogames/embeddedassets/simplespacespritesheet"
	"github.com/ahmedtd/gogames/lib/viewport2"
	"github.com/ahmedtd/gogames/lib/vmath/mat22"
	"github.com/ahmedtd/gogames/lib/vmath/mat33"
	"github.com/ahmedtd/gogames/lib/vmath/vec2"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func main() {
	flag.Parse()

	ctx := context.Background()

	if err := do(ctx); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func do(ctx context.Context) error {
	if err := kenneyrtsmedievalspritesheet.Load(); err != nil {
		return fmt.Errorf("while loading kenneyrtsmedievalspritesheet: %w", err)
	}

	if err := simplespacespritesheet.Load(); err != nil {
		return fmt.Errorf("while loading simplespacespritesheet: %w", err)
	}

	world := BuildInitialWorld()

	r := &Renderer{
		World: world,
	}

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("Walden")
	if err := ebiten.RunGame(r); err != nil {
		return fmt.Errorf("while running game: %w", err)
	}

	return nil
}

type Renderer struct {
	World *World

	InViewJoystickMode                      bool
	CursorPositionAtStartOfViewJoystickMode vec2.T
}

var _ ebiten.Game = (*Renderer)(nil)

func (r *Renderer) Update() error {
	viewport := r.World.PrevState.Viewport
	screenWidth, screenHeight := ebiten.WindowSize()
	viewport.ComputeForFrame(screenWidth, screenHeight)

	xWheel, yWheel := ebiten.Wheel()

	cursorXInt, cursorYInt := ebiten.CursorPosition()
	cursorPosition := vec2.T{float64(cursorXInt), float64(cursorYInt)}

	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		if !r.InViewJoystickMode {
			r.InViewJoystickMode = true
			r.CursorPositionAtStartOfViewJoystickMode = cursorPosition
		}
	} else {
		r.InViewJoystickMode = false
		r.CursorPositionAtStartOfViewJoystickMode = vec2.T{}
	}

	userInput := &UserInput{
		XWheel: xWheel,
		YWheel: yWheel,

		ViewJoystickMode:      r.InViewJoystickMode,
		ViewJoystickWorldFrom: viewport.ScreenPositionToWorldPosition(r.CursorPositionAtStartOfViewJoystickMode),
		ViewJoystickWorldTo:   viewport.ScreenPositionToWorldPosition(cursorPosition),
	}

	r.World.Evolve(userInput)
	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	viewport := r.World.NextState.Viewport
	screenWidth, screenHeight := ebiten.WindowSize()
	viewport.ComputeForFrame(screenWidth, screenHeight)

	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			center := vec2.T{float64(i) + 0.5, float64(j) + 0.5}
			tile := kenneyrtsmedievalspritesheet.TILE_DIRT1
			if (i+j)%2 == 0 {
				tile = kenneyrtsmedievalspritesheet.TILE_GRASS1
			}
			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
				screen,
				tile,
				viewport.WorldToScreenMatrix,
				mat33.Transform2(1.0/128, mat22.Identity(), center),
			)
			tileScreenCenter := mat33.MulMV2(viewport.WorldToScreenMatrix, center)
			vector.DrawFilledCircle(screen, float32(tileScreenCenter[0]), float32(tileScreenCenter[1]), 4, color.RGBA{255, 0, 0, 255}, true)
		}
	}

	for _, ent := range r.World.NextState.Entities {
		switch ent.Type {
		case EntityTypeTree:
			modelToWorld := mat33.Transform2(1.0/128.0, ent.Orientation, ent.Position)
			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
				screen,
				kenneyrtsmedievalspritesheet.LOG,
				viewport.WorldToScreenMatrix,
				modelToWorld,
			)
		}

		if ent.Type != EntityTypeInvalid {
			entScreenCenter := mat33.MulMV2(viewport.WorldToScreenMatrix, ent.Position)
			vector.DrawFilledCircle(screen, float32(entScreenCenter[0]), float32(entScreenCenter[1]), 4, color.RGBA{0, 255, 0, 255}, true)
		}
	}
}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

type World struct {
	PrevState *WorldState
	NextState *WorldState

	NextID int64

	TreeRNG *rand.Rand
}

func BuildInitialWorld() *World {
	initialWorldState := &WorldState{}

	// Place trees
	treeRNG := rand.New(rand.NewChaCha8([32]byte{}))
	for i := 0; i < 30; i++ {
		initialWorldState.Entities = append(initialWorldState.Entities, &Entity{
			Type:        EntityTypeTree,
			Team:        0,
			Orientation: mat22.Rotate(0),
			Position:    vec2.UniformDiskDistribution(treeRNG, 10),
		})
	}

	initialWorldState.Viewport = viewport2.Viewport{
		Center:             vec2.T{0.0, 0.0},
		Orientation:        mat22.Identity(),
		PixelsPerWorldUnit: 50.0,
	}

	return &World{
		PrevState: initialWorldState,
		NextState: initialWorldState.Clone(),
		TreeRNG:   treeRNG,
	}
}

type UserInput struct {
	XWheel float64
	YWheel float64

	ViewJoystickMode      bool
	ViewJoystickWorldFrom vec2.T
	ViewJoystickWorldTo   vec2.T
}

func (w *World) Evolve(userInput *UserInput) {
	w.NextState, w.PrevState = w.PrevState, w.NextState

	w.NextState.CurStep = w.PrevState.CurStep + 1
	w.NextState.CurSimTime = w.PrevState.CurSimTime + 1_000_000_000/60
	w.NextState.CurRealTime = time.Now()

	deltaSec := 1.0 / 60.0

	for i := range w.PrevState.Entities {
		// Do a full direct copy.
		w.NextState.Entities[i].Type = w.PrevState.Entities[i].Type
		w.NextState.Entities[i].Team = w.PrevState.Entities[i].Team
		w.NextState.Entities[i].Orientation = w.PrevState.Entities[i].Orientation
		w.NextState.Entities[i].Position = w.PrevState.Entities[i].Position
	}

	viewportVel := vec2.T{}
	if userInput.ViewJoystickMode {
		viewportVel = vec2.Sub(userInput.ViewJoystickWorldTo, userInput.ViewJoystickWorldFrom)
	}
	w.NextState.Viewport.Center[0] = w.PrevState.Viewport.Center[0] + viewportVel[0]*deltaSec
	w.NextState.Viewport.Center[1] = w.PrevState.Viewport.Center[1] + viewportVel[1]*deltaSec

	w.NextState.Viewport.PixelsPerWorldUnit = w.PrevState.Viewport.PixelsPerWorldUnit
	if userInput.YWheel > 0 {
		w.NextState.Viewport.PixelsPerWorldUnit *= 1.125
	} else if userInput.YWheel < 0 {
		w.NextState.Viewport.PixelsPerWorldUnit /= 1.125
	}
}

type WorldState struct {
	CurStep     int64
	CurSimTime  int64
	CurRealTime time.Time

	Entities []*Entity

	Viewport viewport2.Viewport
}

func (w *WorldState) Clone() *WorldState {
	out := &WorldState{}
	out.CurStep = w.CurStep
	out.CurSimTime = w.CurSimTime
	out.CurRealTime = w.CurRealTime

	out.Entities = make([]*Entity, len(w.Entities))
	for i := range w.Entities {
		if w.Entities[i] == nil {
			continue
		}
		out.Entities[i] = &Entity{
			Type:        w.Entities[i].Type,
			Team:        w.Entities[i].Team,
			Orientation: w.Entities[i].Orientation,
			Position:    w.Entities[i].Position,
		}
	}

	out.Viewport = w.Viewport
	return out
}

type EntityType int

const (
	EntityTypeInvalid EntityType = iota
	EntityTypeTree
	EntityTypeWoman
	EntityTypeMan
	EntityTypeSawmill
)

type Entity struct {
	Type EntityType

	Team int

	Orientation mat22.T
	Position    vec2.T
}
