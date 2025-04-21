package main

// import (
// 	. "github.com/mmcloughlin/avo/build"
// 	. "github.com/mmcloughlin/avo/operand"
// )

// func main() {
// 	TEXT("rayTestAABB", NOSPLIT,
// 		"func(lo [3]float32, hi [3]float32, point [3]float, inverseSlope [3]float) bool")

// 	// 	// https://tavianator.com/2011/ray_box.html

// 	lo := Load(Param("lo"), YMM())
// 	hi := Load(Param("hi"), YMM())
// 	point := Load(Param("point"), YMM())
// 	inverseSlope := Load(Param("inverseSlope"), YMM())

// 	// tx1 := (b.Lo[0] - point[0]) * inverseSlope[0]
// 	// ty1 := (b.Lo[1] - point[1]) * inverseSlope[1]
// 	// tz1 := (b.Lo[2] - point[2]) * inverseSlope[2]
// 	t1 := YMM()
// 	VSUBPS(point, lo, t1) // I think this is t1 = lo - point
// 	VMULPS(t1, inverseSlope, t1)

// 	// tx2 := (b.Hi[0] - point[0]) * inverseSlope[0]
// 	// ty2 := (b.Hi[1] - point[1]) * inverseSlope[1]
// 	// tz2 := (b.Hi[2] - point[2]) * inverseSlope[2]

// 	// tMin := min(tx1, tx2)
// 	// tMax := max(tx1, tx2)

// 	// tMin = max(tMin, min(ty1, ty2))
// 	// tMax = min(tMax, max(ty1, ty2))

// 	// tMin = max(tMin, min(tz1, tz2))
// 	// tMax = min(tMax, max(tz1, tz2))

// 	// return tMin <= tMax

// 	RET()

// 	Generate()
// }
