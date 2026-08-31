package main

import (
	"container/heap"
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/ahmedtd/gogames/lib/vmath/af32"
)

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

	BerryPathCosts *af32.T
}

func (t *Terrain) Clone() *Terrain {
	out := &Terrain{}

	out.MinX = t.MinX
	out.MaxX = t.MaxX
	out.MinY = t.MinY
	out.MaxY = t.MaxY

	out.Tiles = make([]TerrainType, len(t.Tiles))
	copy(out.Tiles, t.Tiles)
	out.BerryPathCosts = af32.Copy(t.BerryPathCosts)

	return out
}

func GenerateTerrain(rng *rand.Rand) *Terrain {
	ter := &Terrain{}

	ter.MinX = -40
	ter.MaxX = 40

	ter.MinY = -40
	ter.MaxY = 40

	ter.Tiles = make([]TerrainType, (ter.MaxX-ter.MinX)*(ter.MaxY-ter.MinY))
	ter.BerryPathCosts = af32.Make(ter.MaxX-ter.MinX, ter.MaxY-ter.MinY)

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

func (t *Terrain) CellCoord(x, y float32) (int, int, bool) {
	if !(t.MinX <= int(x) && int(x) < t.MaxX) {
		return 0, 0, false
	}
	if !(t.MinY <= int(y) && int(y) < t.MaxY) {
		return 0, 0, false
	}

	r := int(y) - t.MinY
	c := int(x) - t.MinX
	return r, c, true
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

// Route computes an A* path between two cell coordinates on this terrain.
// Water and void tiles are impassable. Returns nil, false if no path exists.
func (t *Terrain) Route(from, to CellCoord) (path []CellCoord, ok bool) {
	destTerrain := t.TileAt(to.X, to.Y)
	if destTerrain == TerrainTypeIce || destTerrain == TerrainTypeVoid {
		return nil, false
	}

	if from == to {
		return []CellCoord{to}, true
	}

	gCosts := &Frontier{
		MinX:  t.MinX,
		MaxX:  t.MaxX,
		MinY:  t.MinY,
		MaxY:  t.MaxY,
		Costs: make([]int, (t.MaxX-t.MinX)*(t.MaxY-t.MinY)),
	}
	for i := range gCosts.Costs {
		gCosts.Costs[i] = -1
	}
	gCosts.SetCost(from, 0)

	cameFrom := make(map[CellCoord]CellCoord)
	visited := make(map[CellCoord]bool)

	pq := &CellHeap{}
	heap.Init(pq)

	h0 := math.Abs(float64(to.X-from.X)) + math.Abs(float64(to.Y-from.Y))
	heap.Push(pq, CellHeapEntry{Coord: from, Cost: h0})

	dirs := [4]CellCoord{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

	for pq.Len() > 0 {
		current := heap.Pop(pq).(CellHeapEntry)

		if visited[current.Coord] {
			continue
		}
		visited[current.Coord] = true

		if current.Coord == to {
			var result []CellCoord
			c := to
			for c != from {
				result = append(result, c)
				c = cameFrom[c]
			}
			result = append(result, from)
			for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
				result[i], result[j] = result[j], result[i]
			}
			return result, true
		}

		currentG := gCosts.CostAt(current.Coord)

		for _, d := range dirs {
			neighbor := CellCoord{current.Coord.X + d.X, current.Coord.Y + d.Y}
			if visited[neighbor] {
				continue
			}
			terrain := t.TileAt(neighbor.X, neighbor.Y)
			if terrain == TerrainTypeIce || terrain == TerrainTypeVoid {
				continue
			}

			tentativeG := currentG + 1
			existingG := gCosts.CostAt(neighbor)
			if existingG != -1 && tentativeG >= existingG {
				continue
			}

			gCosts.SetCost(neighbor, tentativeG)
			cameFrom[neighbor] = current.Coord
			nh := math.Abs(float64(to.X-neighbor.X)) + math.Abs(float64(to.Y-neighbor.Y))
			heap.Push(pq, CellHeapEntry{Coord: neighbor, Cost: float64(tentativeG) + nh})
		}
	}

	return nil, false
}
