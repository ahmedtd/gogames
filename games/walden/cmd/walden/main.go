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
	for x := ter.MinX; x < ter.MaxX; x++ {
		for y := ter.MinY; y < ter.MaxY; y++ {
			bl := vec2.T{float64(x), float64(y)}

			tileBox := aabox2.T{
				Lo: vec2.T{bl[0], bl[1]},
				Hi: vec2.T{bl[0] + 1.0, bl[1] + 1.0},
			}

			if !aabox2.Intersects(&viewport.WorldAABox, &tileBox) {
				continue
			}

			tileSprite := -1
			switch ter.TileAt(x, y) {
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
				mat33.Transform2(1.0/128, mat22.Identity(), bl),
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

	ws.Terrain = GenerateTerrain(rng)

	for cellX := ws.Terrain.MinX; cellX < ws.Terrain.MaxX; cellX++ {
		for cellY := ws.Terrain.MinY; cellY < ws.Terrain.MaxY; cellY++ {
			cellCoords := vec2.T{float64(cellX), float64(cellY)}

			terrainType, ok := ws.Terrain.CheckedTileAt(cellX, cellY)
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
						Orientation: mat22.Identity(),
						Position:    pos,
					}
					ws.Entities = append(ws.Entities, ent)
				} else if 100 <= roll && roll < 200 {
					// Spawn berry bush
					pos := vec2.UniformRectDistribution(rng, cellCoords, 1.0, 1.0)
					ent := &Entity{
						Type:        EntityTypeBush,
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
							Type:               EntityTypeMan,
							Orientation:        mat22.Identity(),
							Position:           pos,
							GatherBerriesState: GatherBerriesStateForage,
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
	if subjectPrev.GatherBerriesState == GatherBerriesStateInactive {
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

//go:generate go tool golang.org/x/tools/cmd/stringer -type=TerrainType

type TerrainType int

const (
	TerrainTypeVoid TerrainType = iota
	TerrainTypeDirt
	TerrainTypeGrass
	TerrainTypeSand
	TerrainTypeStone
	TerrainTypeIce
	TerrainTypeMax // Invalid type, only used to determine how many terrain types there are.
)

type Terrain struct {
	MinX, MaxX int
	MinY, MaxY int

	Tiles []TerrainType
}

func GenerateTerrain(rng *rand.Rand) *Terrain {
	ter := &Terrain{}

	ter.MinX = -40
	ter.MaxX = 40

	ter.MinY = -40
	ter.MaxY = 40

	ter.Tiles = make([]TerrainType, (ter.MaxX-ter.MinX)*(ter.MaxY-ter.MinY))

	// Set world border to water
	for x := ter.MinX; x < ter.MaxX; x++ {
		ter.SetTileAt(x, ter.MinY, TerrainTypeIce)
		ter.SetTileAt(x, ter.MaxY-1, TerrainTypeIce)
	}
	for y := ter.MinY; y < ter.MaxY; y++ {
		ter.SetTileAt(ter.MinX, y, TerrainTypeIce)
		ter.SetTileAt(ter.MaxX-1, y, TerrainTypeIce)
	}

	// Seed a few interior water points
	for i := 0; i < ter.NumTiles()/80; i++ {
		x := rng.IntN(ter.MaxX-ter.MinX-2) + ter.MinX + 1
		y := rng.IntN(ter.MaxY-ter.MinY-2) + ter.MinY + 1
		if validForWater(ter, x, y) {
			ter.SetTileAt(x, y, TerrainTypeIce)
		}
	}

	// Seed a few interior grass points
	for i := 0; i < ter.NumTiles()/20; i++ {
		x := rng.IntN(ter.MaxX-ter.MinX-2) + ter.MinX + 1
		y := rng.IntN(ter.MaxY-ter.MinY-2) + ter.MinY + 1
		if validForGrass(ter, x, y) {
			ter.SetTileAt(x, y, TerrainTypeGrass)
		}
	}

	// Seed a few interior dirt points
	for i := 0; i < ter.NumTiles()/40; i++ {
		x := rng.IntN(ter.MaxX-ter.MinX-2) + ter.MinX + 1
		y := rng.IntN(ter.MaxY-ter.MinY-2) + ter.MinY + 1
		if validForDirt(ter, x, y) {
			ter.SetTileAt(x, y, TerrainTypeDirt)
		}
	}

	// Seed a few interior sand points
	for i := 0; i < ter.NumTiles()/40; i++ {
		x := rng.IntN(ter.MaxX-ter.MinX-2) + ter.MinX + 1
		y := rng.IntN(ter.MaxY-ter.MinY-2) + ter.MinY + 1
		if validForSand(ter, x, y) {
			ter.SetTileAt(x, y, TerrainTypeSand)
		}
	}

	for {

		// Build the current frontier
		numFrontierCells := 0
		for x := ter.MinX + 1; x < ter.MaxX-1; x++ {
			for y := ter.MinY + 1; y < ter.MaxY-1; y++ {
				if isOnFrontier(ter, x, y) {
					numFrontierCells++
				}
			}
		}

		if numFrontierCells == 0 {
			break
		}

		// Pick random cell on frontier
		pick := rng.IntN(numFrontierCells)

		frontierX := 0
		frontierY := 0
		curInd := 0
	selectorLoop:
		for x := ter.MinX + 1; x < ter.MaxX-1; x++ {
			for y := ter.MinY + 1; y < ter.MaxY-1; y++ {
				if isOnFrontier(ter, x, y) {
					if curInd == pick {
						frontierX = x
						frontierY = y
						break selectorLoop
					}
					curInd++
				}

			}
		}

		pmf := make([]float64, TerrainTypeMax)
		pmf[TerrainTypeDirt] = dirtWeight(ter, frontierX, frontierY)
		pmf[TerrainTypeGrass] = grassWeight(ter, frontierX, frontierY)
		pmf[TerrainTypeSand] = sandWeight(ter, frontierX, frontierY)
		pmf[TerrainTypeIce] = waterWeight(ter, frontierX, frontierY)

		massSum := 0.0
		for _, mass := range pmf {
			massSum += mass
		}
		if massSum == 0.0 {
			neighborhood := make([]TerrainType, 0, 9)
			for y := frontierY - 1; y <= frontierY+1; y++ {
				for x := frontierX - 1; x <= frontierX+1; x++ {
					neighborhood = append(neighborhood, ter.TileAt(x, y))
				}
			}

			panic(fmt.Sprintf("frontier cell got 0 probability mass for all terrain types x=%v y=%v pmf=%v neighborhood=%v", frontierX, frontierY, pmf, neighborhood))
		}

		ter.SetTileAt(frontierX, frontierY, TerrainType(SampleDiscretePMF(pmf, rng)))
	}

	return ter
}

func SampleDiscretePMF(pmf []float64, rng *rand.Rand) int {
	total := 0.0
	for _, mass := range pmf {
		total += mass
	}
	sample := total * rng.Float64()
	for i, mass := range pmf {
		if sample < mass {
			return i
		}
		sample -= mass
	}
	panic("unreachable")
}

func isOnFrontier(terrain *Terrain, x, y int) bool {
	isVoid := false
	if terrain.TileAt(x, y) == TerrainTypeVoid {
		isVoid = true
	}

	nonVoidNeighbor := false
	for nx := x - 1; nx <= x+1; nx++ {
		for ny := y - 1; ny <= y+1; ny++ {
			if terrain.TileAt(nx, ny) != TerrainTypeVoid {
				nonVoidNeighbor = true
			}
		}
	}

	return isVoid && nonVoidNeighbor
}

func validForGrass(terrain *Terrain, x, y int) bool {
	// Grass may not border sand.
	for nx := x - 1; nx <= x+1; nx++ {
		for ny := y - 1; ny <= y+1; ny++ {
			if terrain.TileAt(nx, ny) == TerrainTypeSand {
				return false
			}
		}
	}
	return true
}

func grassWeight(terrain *Terrain, x, y int) float64 {
	if !validForGrass(terrain, x, y) {
		return 0.0
	}

	weight := 0.0

	// Grass may border dirt
	for nx := x - 1; nx <= x+1; nx++ {
		for ny := y - 1; ny <= y+1; ny++ {
			if terrain.TileAt(nx, ny) == TerrainTypeDirt {
				weight += 0.1
			}
		}
	}

	// Grass likes to border grass
	for nx := x - 1; nx <= x+1; nx++ {
		for ny := y - 1; ny <= y+1; ny++ {
			if terrain.TileAt(nx, ny) == TerrainTypeGrass {
				weight += 1.0
			}
		}
	}

	return weight
}

func validForDirt(terrain *Terrain, x, y int) bool {
	return true
}

func dirtWeight(terrain *Terrain, x, y int) float64 {
	if !validForDirt(terrain, x, y) {
		return 0.0
	}

	weight := 0.0

	// Dirt likes to border dirt
	for nx := x - 1; nx <= x+1; nx++ {
		for ny := y - 1; ny <= y+1; ny++ {
			if terrain.TileAt(nx, ny) == TerrainTypeDirt {
				weight += 1.0
			}
		}
	}

	// Dirt may border sand
	for nx := x - 1; nx <= x+1; nx++ {
		for ny := y - 1; ny <= y+1; ny++ {
			if terrain.TileAt(nx, ny) == TerrainTypeSand {
				weight += 0.1
			}
		}
	}

	// Dirt may border grass
	for nx := x - 1; nx <= x+1; nx++ {
		for ny := y - 1; ny <= y+1; ny++ {
			if terrain.TileAt(nx, ny) == TerrainTypeGrass {
				weight += 0.1
			}
		}
	}

	return weight
}

func validForSand(terrain *Terrain, x, y int) bool {
	// Sand may not border grass.
	for nx := x - 1; nx <= x+1; nx++ {
		for ny := y - 1; ny <= y+1; ny++ {
			if terrain.TileAt(nx, ny) == TerrainTypeGrass {
				return false
			}
		}
	}
	return true
}

func sandWeight(terrain *Terrain, x, y int) float64 {
	if !validForSand(terrain, x, y) {
		return 0.0
	}

	weight := 0.0

	// Sand may border dirt
	for nx := x - 1; nx <= x+1; nx++ {
		for ny := y - 1; ny <= y+1; ny++ {
			if terrain.TileAt(nx, ny) == TerrainTypeDirt {
				weight += 0.1
			}
		}
	}

	// Sand likes to border sand
	for nx := x - 1; nx <= x+1; nx++ {
		for ny := y - 1; ny <= y+1; ny++ {
			if terrain.TileAt(nx, ny) == TerrainTypeSand {
				weight += 1.0
			}
		}
	}

	return weight
}

func validForWater(terrain *Terrain, x, y int) bool {
	return true
}

func waterWeight(terrain *Terrain, x, y int) float64 {
	if !validForWater(terrain, x, y) {
		return 0.0
	}

	weight := 0.0
	// Water likes to border water.
	for nx := x - 1; nx <= x+1; nx++ {
		for ny := y - 1; ny <= y+1; ny++ {
			if terrain.TileAt(nx, ny) == TerrainTypeIce {
				weight += 1.0
			}
		}
	}
	return weight
}

func (t *Terrain) NumTiles() int {
	return (t.MaxX - t.MinX) * (t.MaxY - t.MinY)
}

func (t *Terrain) CheckedTileAt(x, y int) (TerrainType, bool) {
	if !(t.MinX <= x && x < t.MaxX) {
		return TerrainTypeVoid, false
	}
	if !(t.MinY <= y && y < t.MaxY) {
		return TerrainTypeVoid, false
	}

	r := y - t.MinY
	c := x - t.MinX
	return t.Tiles[r*(t.MaxX-t.MinX)+c], true
}

func (t *Terrain) TileAt(x, y int) TerrainType {
	if !(t.MinX <= x && x < t.MaxX) {
		return TerrainTypeVoid
	}
	if !(t.MinY <= y && y < t.MaxY) {
		return TerrainTypeVoid
	}

	r := y - t.MinY
	c := x - t.MinX
	return t.Tiles[r*(t.MaxX-t.MinX)+c]
}

func (t *Terrain) SetTileAt(x, y int, val TerrainType) bool {
	if !(t.MinX <= x && x < t.MaxX) {
		return false
	}
	if !(t.MinY <= y && y < t.MaxY) {
		return false
	}

	r := y - t.MinY
	c := x - t.MinX
	t.Tiles[r*(t.MaxX-t.MinX)+c] = val
	return true
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

	Orientation mat22.T
	Position    vec2.T

	Variant int

	// Allegiance
	Tribe int

	// GatherBerries subsystem
	GatherBerriesState     GatherBerriesState
	GatherBerriesHasTarget bool
	GatherBerriesTarget    int

	AmountBerries float64
}

type GatherBerriesState int

const (
	GatherBerriesStateInactive GatherBerriesState = iota
	GatherBerriesStateForage
	GatherBerriesStateReturn
)

func CopyEnt(src, dst *Entity) {
	dst.Type = src.Type

	dst.Orientation = src.Orientation
	dst.Position = src.Position

	dst.Variant = src.Variant

	dst.Tribe = src.Tribe

	dst.GatherBerriesState = src.GatherBerriesState
	dst.GatherBerriesHasTarget = src.GatherBerriesHasTarget
	dst.GatherBerriesTarget = src.GatherBerriesTarget

	dst.AmountBerries = src.AmountBerries
}
