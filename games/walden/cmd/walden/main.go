package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"time"

	"github.com/ahmedtd/gogames/embeddedassets/fantasyui"
	"github.com/ahmedtd/gogames/embeddedassets/kenneyrtsmedievalspritesheet"
	"github.com/ahmedtd/gogames/embeddedassets/simplespacespritesheet"
	"github.com/ahmedtd/gogames/lib/viewport2"
	"github.com/ahmedtd/gogames/lib/vmath/aabox2"
	"github.com/ahmedtd/gogames/lib/vmath/mat22"
	"github.com/ahmedtd/gogames/lib/vmath/mat33"
	"github.com/ahmedtd/gogames/lib/vmath/vec2"
	"github.com/google/subcommands"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")
	subcommands.Register(&GameCommand{}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}

type GameCommand struct {
	seedString string
}

var _ subcommands.Command = (*GameCommand)(nil)

func (*GameCommand) Name() string {
	return "game"
}

func (*GameCommand) Synopsis() string {
	return "Run the game"
}

func (*GameCommand) Usage() string {
	return ``
}

func (c *GameCommand) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.seedString, "seed", "abcdef", "Seed for world generation")
}

func (c *GameCommand) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if err := c.do(ctx); err != nil {
		log.Printf("Error: %v", err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

func (c *GameCommand) do(ctx context.Context) error {
	if err := kenneyrtsmedievalspritesheet.Load(); err != nil {
		return fmt.Errorf("while loading kenneyrtsmedievalspritesheet: %w", err)
	}

	if err := simplespacespritesheet.Load(); err != nil {
		return fmt.Errorf("while loading simplespacespritesheet: %w", err)
	}

	if err := fantasyui.Load(); err != nil {
		return fmt.Errorf("while loading fantasyui: %w", err)
	}

	seed := sha256.Sum256([]byte(c.seedString))
	world := BuildInitialWorld(seed)

	r := &Renderer{
		World: world,
	}

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("Walden")
	if err := ebiten.RunGameWithOptions(r, &ebiten.RunGameOptions{ScreenTransparent: false}); err != nil {
		return fmt.Errorf("while running game: %w", err)
	}

	return nil
}

type Renderer struct {
	World *World

	InViewJoystickMode                      bool
	CursorPositionAtStartOfViewJoystickMode vec2.T

	ShowEntCenters         bool
	ShowEntCentersDebounce bool
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

	if ebiten.IsKeyPressed(ebiten.KeyF3) {
		if !r.ShowEntCentersDebounce {
			r.ShowEntCentersDebounce = true
			r.ShowEntCenters = !r.ShowEntCenters
		}
	} else {
		r.ShowEntCentersDebounce = false
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

	screen.Fill(color.Black)

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

		if ent.Type != EntityTypeInvalid && r.ShowEntCenters {
			modelToWorld := mat33.Transform2(1.0/256.0, ent.Orientation, ent.Position)
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

	DeltaSec float64

	NextID int64

	TreeRNG *rand.Rand
}

func BuildInitialWorld(seed [32]byte) *World {
	ws := &WorldState{}
	randSource := rand.NewChaCha8(seed)
	rng := rand.New(randSource)

	ws.Terrain = GenerateTerrain()

	for cellX := ws.Terrain.MinX; cellX < ws.Terrain.MaxX; cellX++ {
		for cellY := ws.Terrain.MinY; cellY < ws.Terrain.MaxY; cellY++ {
			cellCoords := vec2.T{float64(cellX), float64(cellY)}

			terrainType, ok := ws.Terrain.TileAt(cellX, cellY)
			if !ok {
				panic("Bad cell")
			}
			switch terrainType {
			case TerrainTypeDirt, TerrainTypeGrass:
				roll := rng.IntN(1000)
				if 0 <= roll && roll < 100 {
					// Spawn tree
					pos := vec2.UniformRectDistribution(rng, cellCoords, 1.0, 1.0)
					ent := &Entity{
						Type:        EntityTypeTree,
						Team:        0,
						Orientation: mat22.Identity(),
						Position:    pos,
					}
					ws.Entities = append(ws.Entities, ent)
				} else if 100 <= roll && roll < 200 {
					// Spawn berry bush
					pos := vec2.UniformRectDistribution(rng, cellCoords, 1.0, 1.0)
					ent := &Entity{
						Type:        EntityTypeBush,
						Team:        0,
						Orientation: mat22.Identity(),
						Position:    pos,

						AmountBerries: rng.Float64() * 30,
					}
					ws.Entities = append(ws.Entities, ent)
				} else if 200 <= roll && roll < 210 {
					// Spawn barn
					pos := vec2.UniformRectDistribution(rng, cellCoords, 1.0, 1.0)
					ent := &Entity{
						Type:        EntityTypeBarn,
						Orientation: mat22.Identity(),
						Position:    pos,
					}
					ws.Entities = append(ws.Entities, ent)

					// Spawn villagers around the barn.
					for i := 0; i < 5; i++ {
						pos := vec2.UniformRectDistribution(rng, cellCoords, 1.0, 1.0)

						ent := &Entity{
							Type:                EntityTypeMan,
							Orientation:         mat22.Identity(),
							Position:            pos,
							GatherBerriesActive: true,
						}

						if rng.IntN(5) == 0 {
							ent.Type = EntityTypeMan
						} else {
							ent.Type = EntityTypeWoman
						}

						ws.Entities = append(ws.Entities, ent)
					}
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
		DeltaSec:  1.0 / 60.0,
		TreeRNG:   rng,
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

	SteadyStatePass(w)
	BerryGrowingPass(w)
	BerryGatheringPass(w)
	ViewportPass(w, userInput)
}

// SteadyStatePass copies enitities previous state to their next states.
func SteadyStatePass(w *World) {
	for i := range w.PrevState.Entities {
		nextEnt := w.NextState.Entities[i]
		prevEnt := w.PrevState.Entities[i]
		CopyEnt(prevEnt, nextEnt)
	}
}

func BerryGrowingPass(w *World) {
	for i := range w.PrevState.Entities {
		subjectPrev := w.PrevState.Entities[i]
		subjectNext := w.NextState.Entities[i]

		if subjectPrev.Type != EntityTypeBush {
			continue
		}

		growAmount := 0.01 / 60.0
		if subjectPrev.AmountBerries > 30.0-growAmount {
			growAmount = 30.0 - subjectPrev.AmountBerries
		}
		subjectNext.AmountBerries = subjectPrev.AmountBerries + growAmount
	}
}

func BerryGatheringPass(w *World) {
	for i := range w.PrevState.Entities {
		subjectPrev := w.PrevState.Entities[i]
		subjectNext := w.NextState.Entities[i]

		if subjectPrev.Type != EntityTypeWoman && subjectPrev.Type != EntityTypeMan {
			continue
		}

		berryGatheringSubj(w, subjectPrev, subjectNext)
	}
}

func berryGatheringSubj(w *World, subjectPrev, subjectNext *Entity) {
	if !subjectPrev.GatherBerriesActive {
		return
	}

	switch subjectPrev.GatherBerriesState {
	case GatherBerriesStateForage:
		if subjectPrev.AmountBerries > 20.0 {
			subjectNext.GatherBerriesState = GatherBerriesStateReturn
			subjectNext.GatherBerriesHasTarget = false
			subjectNext.GatherBerriesTarget = -1
			return
		} else if !subjectPrev.GatherBerriesHasTarget {
			// Pick a target - closest bush with more than 10kg of berries
			bushIndex, _, ok := w.PrevState.ClosestEntityToPoint(subjectPrev.Position, func(ws *WorldState, i int) bool {
				return ws.Entities[i].Type == EntityTypeBush && ws.Entities[i].AmountBerries > 10.0
			})
			if !ok {
				// No bush found, do nothing
				return
			}

			subjectNext.GatherBerriesHasTarget = true
			subjectNext.GatherBerriesTarget = bushIndex

			return
		} else {
			// We have a target.
			bushPrev := w.PrevState.Entities[subjectPrev.GatherBerriesTarget]
			bushNext := w.NextState.Entities[subjectPrev.GatherBerriesTarget]

			bushTo := vec2.Sub(bushPrev.Position, subjectPrev.Position)
			bushDistSquared := vec2.NormSquared(bushTo)
			if bushDistSquared < 0.2 {
				if bushPrev.AmountBerries > 0 {
					// Gather berries
					transferAmount := 1.0 / 60.0
					if bushPrev.AmountBerries < transferAmount {
						transferAmount = bushPrev.AmountBerries
					}
					bushNext.AmountBerries = bushPrev.AmountBerries - transferAmount
					subjectNext.AmountBerries = subjectPrev.AmountBerries + transferAmount
				} else {
					// Pick a new target
					subjectNext.GatherBerriesHasTarget = false
					subjectNext.GatherBerriesTarget = -1
				}

			} else {
				// Move towards bush.
				dir := vec2.MulSV(1.0*w.DeltaSec, vec2.Normalize(bushTo))
				subjectNext.Position = vec2.Add(subjectPrev.Position, dir)
			}

		}

	case GatherBerriesStateReturn:
		if subjectPrev.AmountBerries == 0.0 {
			subjectNext.GatherBerriesState = GatherBerriesStateForage
			subjectNext.GatherBerriesHasTarget = false
			subjectNext.GatherBerriesTarget = -1
		} else if !subjectPrev.GatherBerriesHasTarget {
			depotIndex, _, ok := w.PrevState.ClosestEntityToPoint(subjectPrev.Position, func(ws *WorldState, i int) bool {
				return ws.Entities[i].Type == EntityTypeBarn
			})
			if !ok {
				// No depot found, do nothing.
				return
			}

			subjectNext.GatherBerriesHasTarget = true
			subjectNext.GatherBerriesTarget = depotIndex
		} else {
			// We have a target.
			depotPrev := w.PrevState.Entities[subjectPrev.GatherBerriesTarget]
			depotNext := w.NextState.Entities[subjectPrev.GatherBerriesTarget]

			depotTo := vec2.Sub(depotPrev.Position, subjectPrev.Position)
			depotDistSquared := vec2.NormSquared(depotTo)
			if depotDistSquared < 0.2 && subjectPrev.AmountBerries > 0 {
				// Drop berries
				transferAmount := 1.0 / 60.0
				if subjectPrev.AmountBerries < transferAmount {
					transferAmount = subjectPrev.AmountBerries
				}
				depotNext.AmountBerries = depotPrev.AmountBerries + transferAmount
				subjectNext.AmountBerries = subjectPrev.AmountBerries - transferAmount
			} else {
				// Move towards depot.
				dir := vec2.MulSV(1.0*w.DeltaSec, vec2.Normalize(depotTo))
				subjectNext.Position = vec2.Add(subjectPrev.Position, dir)
			}
		}

	}
}

func ViewportPass(w *World, userInput *UserInput) {
	viewportVel := vec2.T{}
	if userInput.ViewJoystickMode {
		viewportVel = vec2.Sub(userInput.ViewJoystickWorldTo, userInput.ViewJoystickWorldFrom)
	}
	w.NextState.Viewport.Center[0] = w.PrevState.Viewport.Center[0] + viewportVel[0]*w.DeltaSec
	w.NextState.Viewport.Center[1] = w.PrevState.Viewport.Center[1] + viewportVel[1]*w.DeltaSec

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

func (w *WorldState) ClosestEntityToPoint(p vec2.T, filter func(*WorldState, int) bool) (int, float64, bool) {
	minDistSquared := math.Inf(1)
	minEnt := -1
	for i := range w.Entities {
		if !filter(w, i) {
			continue
		}

		distSquared := vec2.NormSquared(vec2.Sub(w.Entities[i].Position, p))
		if distSquared < minDistSquared {
			minDistSquared = distSquared
			minEnt = i
		}
	}

	if minEnt == -1 {
		return -1, 0.0, false
	}

	return minEnt, minDistSquared, true
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
		out.Entities[i] = &Entity{}
		CopyEnt(w.Entities[i], out.Entities[i])
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
	return t.Tiles[r*(t.MaxX-t.MinX)+c], true
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

	// GatherBerries subsystem
	GatherBerriesActive    bool
	GatherBerriesState     GatherBerriesState
	GatherBerriesHasTarget bool
	GatherBerriesTarget    int

	AmountBerries float64
}

type GatherBerriesState int

const (
	GatherBerriesStateForage GatherBerriesState = iota
	GatherBerriesStateReturn
)

func CopyEnt(src, dst *Entity) {
	dst.Type = src.Type
	dst.Team = src.Team
	dst.Orientation = src.Orientation
	dst.Position = src.Position
	dst.Variant = src.Variant

	dst.GatherBerriesActive = src.GatherBerriesActive
	dst.GatherBerriesState = src.GatherBerriesState
	dst.GatherBerriesHasTarget = src.GatherBerriesHasTarget
	dst.GatherBerriesTarget = src.GatherBerriesTarget

	dst.AmountBerries = src.AmountBerries
}
