package aabox2

import (
	"github.com/ahmedtd/gogames/lib/vmath/vec2"
)

type T struct {
	Lo vec2.T
	Hi vec2.T
}

func OfPoints(points ...vec2.T) T {
	if len(points) == 0 {
		return T{}
	}

	ret := T{
		Lo: points[0],
		Hi: points[0],
	}
	for _, point := range points {
		for i := range 2 {
			if point[i] < ret.Lo[i] {
				ret.Lo[i] = point[i]
			}
			if point[i] > ret.Hi[i] {
				ret.Hi[i] = point[i]
			}
		}
	}
	return ret
}

func Intersects(a, b *T) bool {
	return !(a.Lo[0] > b.Hi[0] ||
		b.Lo[0] > a.Hi[0] ||
		a.Lo[1] > b.Hi[1] ||
		b.Lo[1] > a.Hi[1])
}
