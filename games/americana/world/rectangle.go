package world

import "shellworld/lib/vmath/vec2"

type Rectangle struct {
	Min vec2.T
	Max vec2.T
}

func (r *Rectangle) Contains(point vec2.T) bool {
	return point[0] >= r.Min[0] && point[0] < r.Max[0] && point[1] >= r.Min[1] && point[1] < r.Max[1]
}
