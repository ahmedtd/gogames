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
	"github.com/ahmedtd/gogames/lib/vmath/aabox2"
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

	r.drawTerrain(screen, viewport, r.World.NextState.Terrain)

	for _, ent := range r.World.NextState.Entities {
		switch ent.Type {
		case EntityTypeTree:
			modelToWorld := mat33.Transform2(1.0/128.0, ent.Orientation, ent.Position)
			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
				screen,
				kenneyrtsmedievalspritesheet.TREE_CONIFER_LARGE,
				viewport.WorldToScreenMatrix,
				modelToWorld,
			)
		}

		if ent.Type != EntityTypeInvalid {
			entScreenCenter := mat33.MulMV2(viewport.WorldToScreenMatrix, ent.Position)
			vector.DrawFilledCircle(screen, float32(entScreenCenter[0]), float32(entScreenCenter[1]), 4, color.RGBA{0, 255, 0, 255}, true)
		}
	}

	originScreenCenter := mat33.MulMV2(viewport.WorldToScreenMatrix, vec2.T{0, 0})
	vector.DrawFilledCircle(screen, float32(originScreenCenter[0]), float32(originScreenCenter[1]), 4, color.RGBA{0, 0, 255, 255}, true)
}

func (r *Renderer) drawTerrain(screen *ebiten.Image, viewport viewport2.Viewport, ter *Terrain) {
	for r := 0; r < ter.MaxY-ter.MinY; r++ {
		for c := 0; c < ter.MaxX-ter.MinX; c++ {
			center := vec2.T{float64(r+ter.MinY) + 0.5, float64(c+ter.MinX) + 0.5}

			tileBox := aabox2.T{
				Lo: vec2.T{center[0] - 0.5, center[1] - 0.5},
				Hi: vec2.T{center[0] + 0.5, center[1] + 0.5},
			}

			if !aabox2.Intersects(&viewport.WorldAABox, &tileBox) {
				continue
			}

			tileIndex := r*(ter.MaxX-ter.MinX) + c

			tileSprite := -1
			switch ter.Tiles[tileIndex] {
			case TerrainTypeDirt:
				tileSprite = kenneyrtsmedievalspritesheet.TILE_DIRT1
			case TerrainTypeGrass:
				tileSprite = kenneyrtsmedievalspritesheet.TILE_GRASS1
			case TerrainTypeSand:
				tileSprite = kenneyrtsmedievalspritesheet.TILE_SAND1
			case TerrainTypeStone:
				tileSprite = kenneyrtsmedievalspritesheet.TILE_ROCK1
			case TerrainTypeIce:
				tileSprite = kenneyrtsmedievalspritesheet.TILE_ICE1
			}

			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
				screen,
				tileSprite,
				viewport.WorldToScreenMatrix,
				mat33.Transform2(1.0/128, mat22.Identity(), center),
			)
			//tileScreenCenter := mat33.MulMV2(viewport.WorldToScreenMatrix, center)
			//vector.DrawFilledCircle(screen, float32(tileScreenCenter[0]), float32(tileScreenCenter[1]), 4, color.RGBA{255, 0, 0, 255}, true)
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

	initialWorldState.Terrain = GenerateTerrain()

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

	Terrain *Terrain

	Entities []*Entity

	Viewport viewport2.Viewport
}

func (w *WorldState) Clone() *WorldState {
	out := &WorldState{}
	out.CurStep = w.CurStep
	out.CurSimTime = w.CurSimTime
	out.CurRealTime = w.CurRealTime

	out.Terrain = w.Terrain.Clone()

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

type TerrainType int

const (
	TerrainTypeDirt TerrainType = iota
	TerrainTypeGrass
	TerrainTypeSand
	TerrainTypeStone
	TerrainTypeIce
)

type Terrain struct {
	MinX, MaxX int
	MinY, MaxY int

	Tiles []TerrainType
}

func GenerateTerrain() *Terrain {
	ret := &Terrain{}

	ret.MinX = -20
	ret.MaxX = 20

	ret.MinY = -20
	ret.MaxY = 20

	ret.Tiles = make([]TerrainType, (ret.MaxX-ret.MinX)*(ret.MaxY-ret.MinY))
	return ret
}

func (t *Terrain) Clone() *Terrain {
	out := &Terrain{}

	out.MinX = t.MinX
	out.MaxX = t.MaxX
	out.MinY = t.MinY
	out.MaxY = t.MaxY

	out.Tiles = make([]TerrainType, len(t.Tiles))
	copy(out.Tiles, t.Tiles)

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
