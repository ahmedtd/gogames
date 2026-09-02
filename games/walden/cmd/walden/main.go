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
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var (
	F32InfP = float32(math.Inf(1))
	F32InfN = float32(math.Inf(-1))
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

	ShowBerryGradient         bool
	ShowBerryGradientDebounce bool
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

	if ebiten.IsKeyPressed(ebiten.KeyF4) {
		if !r.ShowBerryGradientDebounce {
			r.ShowBerryGradientDebounce = true
			r.ShowBerryGradient = !r.ShowBerryGradient
		}
	} else {
		r.ShowBerryGradientDebounce = false
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
				ent.Tribe.WomanSprite(),
				viewport.WorldToScreenMatrix,
				modelToWorld,
			)
		case EntityTypeMan:
			modelToWorld := mat33.Transform2(1.0/128.0, ent.Orientation, ent.Position)
			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPosition(
				screen,
				ent.Tribe.ManSprite(),
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
			kenneyrtsmedievalspritesheet.Sheet.RenderAtWorldPositionTinted(
				screen,
				kenneyrtsmedievalspritesheet.BARN,
				viewport.WorldToScreenMatrix,
				modelToWorld,
				ent.Tribe.Tint(),
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

			if r.ShowBerryGradient {
				r.drawBerryGradientTile(screen, viewport, ter, x, y, tileBox)
			}
		}
	}
}

// berryGradientDisplayMaxCost is the path cost (in cells) at which the
// F4 debug overlay saturates to fully red.
const berryGradientDisplayMaxCost = 20.0

// drawBerryGradientTile draws a translucent red overlay on tile (x, y),
// with opacity proportional to the tile's precomputed cost-to-nearest-bush
// (Terrain.BerryPathCosts). This is a debug visualization for the F4 key,
// toggled via Renderer.ShowBerryGradient.
func (r *Renderer) drawBerryGradientTile(screen *ebiten.Image, viewport viewport2.Viewport, ter *Terrain, x, y int, tileBox aabox2.T) {
	cost := ter.BerryPathCostAt(x, y)
	if cost <= 0 {
		return
	}

	normalizedCost := float64(cost) / berryGradientDisplayMaxCost
	if normalizedCost > 1.0 || math.IsInf(float64(cost), 1) {
		normalizedCost = 1.0
	}

	tint := color.RGBA{0xff, 0x00, 0x00, uint8(normalizedCost * 200)}

	p0 := viewport.WorldPositionToScreenPosition(tileBox.Lo)
	p1 := viewport.WorldPositionToScreenPosition(tileBox.Hi)

	minX := math.Min(p0[0], p1[0])
	maxX := math.Max(p0[0], p1[0])
	minY := math.Min(p0[1], p1[1])
	maxY := math.Max(p0[1], p1[1])

	vector.DrawFilledRect(screen, float32(minX), float32(minY), float32(maxX-minX), float32(maxY-minY), tint, false)
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

	// Barn spawn points are recorded here as we scan the terrain, and used
	// to spawn each barn's villagers only once every barn has been placed.
	type barnSpawn struct {
		cellCoords vec2.T
		tribe      Tribe
	}
	var barnSpawns []barnSpawn

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
						Type:            EntityTypeTree,
						Orientation:     mat22.Identity(),
						Position:        pos,
						CollisionRadius: TreeCollisionRadius,
					}
					ws.Entities = append(ws.Entities, ent)
				} else if 100 <= roll && roll < 200 {
					// Spawn berry bush
					pos := vec2.UniformRectDistribution(rng, cellCoords, 1.0, 1.0)
					ent := &Entity{
						Type:            EntityTypeBush,
						Orientation:     mat22.Identity(),
						Position:        pos,
						CollisionRadius: BushCollisionRadius,

						AmountBerries: rng.Float64() * 30,
					}
					ws.Entities = append(ws.Entities, ent)
				} else if 200 <= roll && roll < 210 {
					// Spawn barn
					tribe := Tribe(1 + rng.IntN(int(TribeMax)-1))
					pos := vec2.UniformRectDistribution(rng, cellCoords, 1.0, 1.0)
					ent := &Entity{
						Type:        EntityTypeBarn,
						Orientation: mat22.Identity(),
						Position:    pos,
						Tribe:       tribe,
					}
					ws.Entities = append(ws.Entities, ent)

					barnSpawns = append(barnSpawns, barnSpawn{cellCoords: cellCoords, tribe: tribe})
				} else {
					// Spawn nothing
				}
			}
		}
	}

	// Now that every barn has been placed, spawn each barn's villagers,
	// belonging to the same tribe as their barn.
	for _, spawn := range barnSpawns {
		for i := 0; i < 5; i++ {
			pos := vec2.UniformRectDistribution(rng, spawn.cellCoords, 1.0, 1.0)

			ent := &Entity{
				Type:               EntityTypeWoman,
				Orientation:        mat22.Identity(),
				Position:           pos,
				Tribe:              spawn.tribe,
				CollisionRadius:    PersonCollisionRadius,
				GatherBerriesState: GatherBerriesStateForage,
			}

			ws.Entities = append(ws.Entities, ent)
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
	BerryRoutingPass(w)
	BarnRoutingPass(w)
	BerryGrowingPass(w)
	BerryGatheringPass(w)
	CollisionPass(w)
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

// BerryRoutingPass computes, for every terrain cell, the cost of the
// shortest path to the closest berry bush worth foraging from.
func BerryRoutingPass(w *World) {
	terrain := w.NextState.Terrain

	var seeds []CellCoord
	for i := range w.PrevState.Entities {
		if w.PrevState.Entities[i].Type != EntityTypeBush {
			continue
		}
		// Only route towards bushes worth foraging from — matches the
		// filter berryGatheringSubj uses when picking a target.
		if w.PrevState.Entities[i].AmountBerries <= 10.0 {
			continue
		}

		seeds = append(seeds, worldToCell(w.PrevState.Entities[i].Position))
	}

	terrain.FloodFill(terrain.BerryPathCosts, seeds)
}

// BarnRoutingPass computes, for every terrain cell, the cost of the
// shortest path to the closest barn.
func BarnRoutingPass(w *World) {
	terrain := w.NextState.Terrain

	var seeds []CellCoord
	for i := range w.PrevState.Entities {
		if w.PrevState.Entities[i].Type != EntityTypeBarn {
			continue
		}

		seeds = append(seeds, worldToCell(w.PrevState.Entities[i].Position))
	}

	terrain.FloodFill(terrain.BarnPathCosts, seeds)
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

// CollisionPass gently nudges women and men away from anything within their
// collision radius (trees, bushes, and other people). Trees and bushes
// still participate in collision — they push people away — but they are
// never pushed themselves, since only women and men are corrected here.
func CollisionPass(w *World) {
	for i := range w.PrevState.Entities {
		subjectPrev := w.PrevState.Entities[i]
		if subjectPrev.Type != EntityTypeWoman && subjectPrev.Type != EntityTypeMan {
			continue
		}
		if subjectPrev.CollisionRadius <= 0 {
			continue
		}

		push := vec2.T{}
		for j := range w.PrevState.Entities {
			if j == i {
				continue
			}

			otherPrev := w.PrevState.Entities[j]
			if otherPrev.CollisionRadius <= 0 {
				continue
			}

			delta := vec2.Sub(subjectPrev.Position, otherPrev.Position)
			minDist := subjectPrev.CollisionRadius + otherPrev.CollisionRadius
			distSquared := vec2.NormSquared(delta)
			if distSquared >= minDist*minDist {
				continue
			}

			dist := math.Sqrt(distSquared)
			dir := vec2.T{1, 0}
			if dist > 1e-6 {
				dir = vec2.MulSV(1.0/dist, delta)
			}

			overlap := minDist - dist
			push = vec2.Add(push, vec2.MulSV(overlap*overlap, dir))
		}

		if vec2.NormSquared(push) > 0.005 {
			subjectNext := w.NextState.Entities[i]
			subjectNext.Position = vec2.Add(subjectNext.Position, vec2.MulSV(0.5, push))
		}
	}
}

// gradientDescentDir looks at the current cell's neighbors in a precomputed
// cost field (costAt) and returns the direction towards whichever neighbor
// is closer to the field's target. Returns ok=false if pos is already
// sitting on a target cell (cost <= 0) or if no route is known from here
// (cost is +Inf all around).
func gradientDescentDir(pos vec2.T, costAt func(x, y int) float32) (vec2.T, bool) {
	cell := worldToCell(pos)
	bestCost := costAt(cell.X, cell.Y)
	if bestCost <= 0 {
		return vec2.T{}, false
	}

	bestCell := cell
	found := false
	dirs := [4]CellCoord{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for _, d := range dirs {
		neighbor := CellCoord{cell.X + d.X, cell.Y + d.Y}
		cost := costAt(neighbor.X, neighbor.Y)
		if cost < bestCost {
			bestCost = cost
			bestCell = neighbor
			found = true
		}
	}
	if !found {
		return vec2.T{}, false
	}

	return vec2.Sub(cellCenter(bestCell), pos), true
}

// moveByGradientDescent advances the entity downhill along a precomputed
// cost field, falling back to straight-line movement if the field has no
// usable gradient at the entity's position.
func moveByGradientDescent(w *World, subjectPrev, subjectNext *Entity, fallbackDir vec2.T, costAt func(x, y int) float32) {
	dir, ok := gradientDescentDir(subjectPrev.Position, costAt)
	if !ok {
		dir = fallbackDir
	}

	if vec2.NormSquared(dir) > 0 {
		step := vec2.MulSV(w.DeltaSec, vec2.Normalize(dir))
		subjectNext.Position = vec2.Add(subjectPrev.Position, step)
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
			bushIndex, _, ok := w.PrevState.ClosestEntityToPoint(subjectPrev.Position, func(ws *WorldState, i int) bool {
				return ws.Entities[i].Type == EntityTypeBush && ws.Entities[i].AmountBerries > 10.0
			})
			if !ok {
				return
			}

			subjectNext.GatherBerriesHasTarget = true
			subjectNext.GatherBerriesTarget = bushIndex
			return
		} else {
			bushPrev := w.PrevState.Entities[subjectPrev.GatherBerriesTarget]
			bushNext := w.NextState.Entities[subjectPrev.GatherBerriesTarget]

			bushTo := vec2.Sub(bushPrev.Position, subjectPrev.Position)
			bushDistSquared := vec2.NormSquared(bushTo)
			if bushDistSquared < 0.2 {
				if bushPrev.AmountBerries > 0 {
					transferAmount := 1.0 / 60.0
					if bushPrev.AmountBerries < transferAmount {
						transferAmount = bushPrev.AmountBerries
					}
					bushNext.AmountBerries = bushPrev.AmountBerries - transferAmount
					subjectNext.AmountBerries = subjectPrev.AmountBerries + transferAmount
				} else {
					subjectNext.GatherBerriesHasTarget = false
					subjectNext.GatherBerriesTarget = -1
				}
			} else {
				moveByGradientDescent(w, subjectPrev, subjectNext, bushTo, w.PrevState.Terrain.BerryPathCostAt)
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
				return
			}

			subjectNext.GatherBerriesHasTarget = true
			subjectNext.GatherBerriesTarget = depotIndex
		} else {
			depotPrev := w.PrevState.Entities[subjectPrev.GatherBerriesTarget]
			depotNext := w.NextState.Entities[subjectPrev.GatherBerriesTarget]

			depotTo := vec2.Sub(depotPrev.Position, subjectPrev.Position)
			depotDistSquared := vec2.NormSquared(depotTo)
			if depotDistSquared < 0.2 && subjectPrev.AmountBerries > 0 {
				transferAmount := 1.0 / 60.0
				if subjectPrev.AmountBerries < transferAmount {
					transferAmount = subjectPrev.AmountBerries
				}
				depotNext.AmountBerries = depotPrev.AmountBerries + transferAmount
				subjectNext.AmountBerries = subjectPrev.AmountBerries - transferAmount
			} else {
				moveByGradientDescent(w, subjectPrev, subjectNext, depotTo, w.PrevState.Terrain.BarnPathCostAt)
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

type CellCoord struct {
	X, Y int
}

func worldToCell(p vec2.T) CellCoord {
	return CellCoord{int(math.Floor(p[0])), int(math.Floor(p[1]))}
}

func cellCenter(c CellCoord) vec2.T {
	return vec2.T{float64(c.X) + 0.5, float64(c.Y) + 0.5}
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

// Collision radii for the entity types that participate in collision.
const (
	TreeCollisionRadius   = 0.3
	BushCollisionRadius   = 0.3
	PersonCollisionRadius = 0.2
)

// Tribe is the faction an entity belongs to. It's only meaningful for
// EntityTypeBarn, EntityTypeMan, and EntityTypeWoman.
type Tribe int

const (
	TribeInvalid Tribe = iota
	TribeGreen
	TribeBlue
	TribeRed
	TribeMax // Invalid, only used to determine how many tribes there are.
)

// Tint returns the color used to recolor this tribe's entities when
// rendering, for sprites that don't have a dedicated per-tribe variant
// (e.g. the barn).
func (t Tribe) Tint() color.Color {
	switch t {
	case TribeGreen:
		return color.RGBA{0x40, 0xc0, 0x40, 0xff}
	case TribeBlue:
		return color.RGBA{0x40, 0x80, 0xff, 0xff}
	case TribeRed:
		return color.RGBA{0xe0, 0x40, 0x40, 0xff}
	default:
		return color.White
	}
}

// WomanSprite returns the spritesheet index for this tribe's woman sprite.
func (t Tribe) WomanSprite() int {
	switch t {
	case TribeGreen:
		return kenneyrtsmedievalspritesheet.GREEN_WOMAN
	case TribeBlue:
		return kenneyrtsmedievalspritesheet.BLUE_WOMAN
	case TribeRed:
		return kenneyrtsmedievalspritesheet.RED_WOMAN
	default:
		return kenneyrtsmedievalspritesheet.GREY_WOMAN
	}
}

// ManSprite returns the spritesheet index for this tribe's man sprite.
func (t Tribe) ManSprite() int {
	switch t {
	case TribeGreen:
		return kenneyrtsmedievalspritesheet.GREEN_MAN
	case TribeBlue:
		return kenneyrtsmedievalspritesheet.BLUE_MAN
	case TribeRed:
		return kenneyrtsmedievalspritesheet.RED_MAN
	default:
		return kenneyrtsmedievalspritesheet.GREY_MAN
	}
}

type Entity struct {
	Type EntityType

	Orientation mat22.T
	Position    vec2.T

	Variant int

	// Allegiance
	Tribe Tribe

	// CollisionRadius is the radius of this entity's collision circle. A
	// value of 0 means the entity does not participate in collision at all.
	// Only EntityTypeTree, EntityTypeBush, EntityTypeWoman, and
	// EntityTypeMan set this to a nonzero value.
	CollisionRadius float64

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

	dst.CollisionRadius = src.CollisionRadius

	dst.GatherBerriesState = src.GatherBerriesState
	dst.GatherBerriesHasTarget = src.GatherBerriesHasTarget
	dst.GatherBerriesTarget = src.GatherBerriesTarget

	dst.AmountBerries = src.AmountBerries
}
