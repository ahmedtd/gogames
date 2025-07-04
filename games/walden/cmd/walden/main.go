package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"github.com/ahmedtd/gogames/embeddedassets/fantasyui"
	"github.com/ahmedtd/gogames/embeddedassets/kenneyrtsmedievalspritesheet"
	"github.com/ahmedtd/gogames/embeddedassets/simplespacespritesheet"
	"github.com/ahmedtd/gogames/lib/viewport2"
	"github.com/ahmedtd/gogames/lib/vmath/aabox2"
	"github.com/ahmedtd/gogames/lib/vmath/mat22"
	"github.com/ahmedtd/gogames/lib/vmath/mat33"
	"github.com/ahmedtd/gogames/lib/vmath/vec2"

	"github.com/hajimehoshi/ebiten/v2"
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

	if err := fantasyui.Load(); err != nil {
		return fmt.Errorf("while loading fantasyui: %w", err)
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
			var sprite int
			switch ent.Variant {
			case 0:
				sprite = kenneyrtsmedievalspritesheet.TREE_SMALL
			case 1:
				sprite = kenneyrtsmedievalspritesheet.TREE_TALL
			case 2:
				sprite = kenneyrtsmedievalspritesheet.TREE_CONIFER
			case 3:
				sprite = kenneyrtsmedievalspritesheet.TREE_CONIFER_LARGE
			}
			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
				screen,
				sprite,
				viewport.WorldToScreenMatrix,
				modelToWorld,
			)
		case EntityTypeBush:
			modelToWorld := mat33.Transform2(1.0/128.0, ent.Orientation, ent.Position)
			if ent.AmountBerries > 0 {
				kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
					screen,
					kenneyrtsmedievalspritesheet.BUSH_BERRIES,
					viewport.WorldToScreenMatrix,
					modelToWorld,
				)
			} else {
				kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
					screen,
					kenneyrtsmedievalspritesheet.BUSH,
					viewport.WorldToScreenMatrix,
					modelToWorld,
				)
			}
		case EntityTypeWoman:
			modelToWorld := mat33.Transform2(1.0/128.0, ent.Orientation, ent.Position)
			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
				screen,
				kenneyrtsmedievalspritesheet.GREY_WOMAN,
				viewport.WorldToScreenMatrix,
				modelToWorld,
			)
		case EntityTypeMan:
			modelToWorld := mat33.Transform2(1.0/128.0, ent.Orientation, ent.Position)
			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
				screen,
				kenneyrtsmedievalspritesheet.GREY_MAN,
				viewport.WorldToScreenMatrix,
				modelToWorld,
			)
		case EntityTypeHouse:
			modelToWorld := mat33.Transform2(1.0/128.0, ent.Orientation, ent.Position)
			var sprite int
			switch ent.Variant {
			case 0:
				sprite = kenneyrtsmedievalspritesheet.HOUSE1
			case 1:
				sprite = kenneyrtsmedievalspritesheet.HOUSE2
			}
			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
				screen,
				sprite,
				viewport.WorldToScreenMatrix,
				modelToWorld,
			)
		case EntityTypeBarn:
			modelToWorld := mat33.Transform2(1.0/128.0, ent.Orientation, ent.Position)
			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
				screen,
				kenneyrtsmedievalspritesheet.BARN,
				viewport.WorldToScreenMatrix,
				modelToWorld,
			)
		}

		if ent.Type != EntityTypeInvalid {
			modelToWorld := mat33.Transform2(1.0/128.0, ent.Orientation, ent.Position)
			fantasyui.Sheet.RenderAtWorldPosition(screen, fantasyui.MINIMAP_ICON_JEWEL_REDIMG, viewport.WorldToScreenMatrix, modelToWorld)
		}
	}
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

func BuildInitialWorld(seed [32]byte) *World {
	ws := &WorldState{}
	randSource := rand.NewChaCha8(seed)
	rng := rand.New(randSource)

	ws.Terrain = GenerateTerrain()

	// Place trees
	for i := 0; i < 30; i++ {
		ws.Entities = append(ws.Entities, &Entity{
			Type:        EntityTypeTree,
			Team:        0,
			Orientation: mat22.Rotate(0),
			Position:    vec2.UniformDiskDistribution(rng, 10),
		})
	}

	// Place berry bushes.
	for cellX := ws.Terrain.MinX; cellX < ws.Terrain.MaxX; cellX++ {
		for cellY := ws.Terrain.MinY; cellY < ws.Terrain.MaxY; cellY++ {
			terrainType, ok := ws.Terrain.TileAt(cellX, cellY)
			if !ok {
				panic("Bad cell")
			}
			switch terrainType {
			case TerrainTypeDirt:
				roll := rng.IntN(1000)
				if 0 <= roll && roll < 100 {
					// Spawn tree
				} else if 100 <= roll && roll < 200 {
					ent := &Entity{
						Type: EntityTypeBush,
						Team: 0,
					}
					ws.Entities = append(ws.Entities, ent)
				} else {
					// Spawn nothing
				}
			}
		}
	}

	ws.Viewport = viewport2.Viewport{
		Center:             vec2.T{0.0, 0.0},
		Orientation:        mat22.Identity(),
		PixelsPerWorldUnit: 50.0,
	}

	return &World{
		PrevState: ws,
		NextState: ws.Clone(),
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
			Type:          w.Entities[i].Type,
			Team:          w.Entities[i].Team,
			Orientation:   w.Entities[i].Orientation,
			Position:      w.Entities[i].Position,
			Variant:       w.Entities[i].Variant,
			AmountBerries: w.Entities[i].AmountBerries,
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

func (t *Terrain) TileAt(x, y int) (TerrainType, bool) {
	if !(t.MinX <= x && x < t.MaxX) {
		return TerrainTypeDirt, false
	}
	if !(t.MinY <= y && y < t.MaxY) {
		return TerrainTypeDirt, false
	}

	r := y - t.MinY
	c := x - t.MinX
	return t.Tiles[r*(t.MaxX-t.MinX)+c]
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
	EntityTypeBush
	EntityTypeWoman
	EntityTypeMan
	EntityTypeHouse
	EntityTypeBarn
)

type Entity struct {
	Type EntityType

	Team int

	Orientation mat22.T
	Position    vec2.T

	Variant int

	AmountBerries int
}
