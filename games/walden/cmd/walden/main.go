package main

import (
	"context"
	"flag"
	"fmt"
	"image/color"
	"log"
	"math/rand/v2"
	"shellworld/embeddedassets/kenneyrtsmedievalspritesheet"
	"shellworld/embeddedassets/simplespacespritesheet"
	"shellworld/lib/viewport2"
	"shellworld/lib/vmath/mat22"
	"shellworld/lib/vmath/mat33"
	"shellworld/lib/vmath/vec2"
	"time"

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
}

var _ ebiten.Game = (*Renderer)(nil)

func (r *Renderer) Update() error {
	userInput := &UserInput{
		KeyWPressed: ebiten.IsKeyPressed(ebiten.KeyW),
		KeySPressed: ebiten.IsKeyPressed(ebiten.KeyS),
		KeyAPressed: ebiten.IsKeyPressed(ebiten.KeyA),
		KeyDPressed: ebiten.IsKeyPressed(ebiten.KeyD),
	}

	r.World.Evolve(userInput)
	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	viewport := r.World.NextState.Viewport
	viewport.ScreenWidthPixels, viewport.ScreenHeightPixels = ebiten.WindowSize()
	worldToScreen := viewport.WorldToScreenMatrix()

	for _, ent := range r.World.NextState.Entities {
		switch ent.Type {
		case EntityTypeTree:
			modelToWorld := mat33.Transform2(1.0/64.0, ent.Orientation, ent.Position)
			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(screen, kenneyrtsmedievalspritesheet.BLUE_MAN, worldToScreen, modelToWorld)
		}

		if ent.Type != EntityTypeInvalid {
			entScreenCenter := mat33.MulMV2(worldToScreen, ent.Position)
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
	KeyWPressed bool
	KeySPressed bool
	KeyAPressed bool
	KeyDPressed bool
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
	if userInput.KeyWPressed {
		viewportVel = vec2.Add(viewportVel, vec2.T{0, 1})
	}
	if userInput.KeySPressed {
		viewportVel = vec2.Add(viewportVel, vec2.T{0, -1})
	}
	if userInput.KeyAPressed {
		viewportVel = vec2.Add(viewportVel, vec2.T{-1, 0})
	}
	if userInput.KeyDPressed {
		viewportVel = vec2.Add(viewportVel, vec2.T{1, 0})
	}
	w.NextState.Viewport.Center[0] = w.PrevState.Viewport.Center[0] + viewportVel[0]*deltaSec
	w.NextState.Viewport.Center[1] = w.PrevState.Viewport.Center[1] + viewportVel[1]*deltaSec
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
