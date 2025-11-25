package pathing

type IntCoord struct {
	X, Y int
}

type IntCoordHeap struct {
	Coords []IntCoord
	Costs  []float64
}

func (h *IntCoordHeap) Push(coord IntCoord, cost float64) {
	for i, c := range h.Coords {
		if c == coord && cost < h.Costs[i] {
			h.Costs[i]*
		}
	}

	h.Coords = append(h.Coords, coord)
	h.Costs = append(h.Costs, cost)
}
