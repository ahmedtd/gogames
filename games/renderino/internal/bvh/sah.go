package bvh

import (
	"math"
	"slices"
)

// splitSAH reorders elements and divides them into two non-empty halves,
// elements[:mid] and elements[mid:], choosing the split with the lowest
// surface area heuristic cost.  Every axis is tried, and along each one,
// every split position between elements sorted by their bounds' centroids.
//
// A split's cost is the surface area of each half's bounds times its number
// of elements, summed over both halves.  A ray that passes through the
// parent's bounds hits each half's bounds with probability proportional to
// its surface area, so this is proportional to the expected work below the
// parent.  (The parent's own surface area divides every candidate's cost
// equally, so it's left out.)
//
// If t.LeafSize is more than 1, elements end up gathered into leaves of up
// to LeafSize, and the work below each half is per leaf rather than per
// element.  So a half's cost counts the leaves it needs, and only splits
// leaving a multiple of LeafSize elements in the first half are considered,
// so that only the last leaf under any node can be partly full.
//
// elements must hold more than max(1, t.LeafSize) elements.
func (t *Tree) splitSAH(elements []*Node) (mid int) {
	n := len(elements)

	// rightArea[i] is the surface area of the bounds of elements[i:].
	t.sahScratch = slices.Grow(t.sahScratch[:0], n)[:n]
	rightArea := t.sahScratch

	leafSize := max(1, t.LeafSize)
	numLeaves := func(count int) float32 {
		return float32((count + leafSize - 1) / leafSize)
	}

	bestAxis, bestMid := 0, leafSize*(n/leafSize/2)
	if bestMid == 0 {
		bestMid = leafSize
	}
	bestCost := float32(math.Inf(1))
	for axis := range 3 {
		sortByCentroid(elements, axis)

		lo, hi := elements[n-1].Lo, elements[n-1].Hi
		for i := n - 1; i >= 1; i-- {
			for k := range 3 {
				lo[k] = min(lo[k], elements[i].Lo[k])
				hi[k] = max(hi[k], elements[i].Hi[k])
			}
			rightArea[i] = surfaceArea(lo[0], lo[1], lo[2], hi[0], hi[1], hi[2])
		}

		lo, hi = elements[0].Lo, elements[0].Hi
		for i := 1; i < n; i++ {
			for k := range 3 {
				lo[k] = min(lo[k], elements[i-1].Lo[k])
				hi[k] = max(hi[k], elements[i-1].Hi[k])
			}
			if i%leafSize != 0 {
				continue
			}
			leftArea := surfaceArea(lo[0], lo[1], lo[2], hi[0], hi[1], hi[2])
			cost := leftArea*numLeaves(i) + rightArea[i]*numLeaves(n-i)
			if cost < bestCost {
				bestAxis, bestMid, bestCost = axis, i, cost
			}
		}
	}

	// The elements are currently sorted along the last axis tried.
	if bestAxis != 2 {
		sortByCentroid(elements, bestAxis)
	}
	return bestMid
}

// sortByCentroid sorts elements by the centroid of their bounds along axis.
func sortByCentroid(elements []*Node, axis int) {
	slices.SortFunc(elements, func(a, b *Node) int {
		// Comparing Lo+Hi is equivalent to comparing (Lo+Hi)/2.
		diff := (a.Lo[axis] + a.Hi[axis]) - (b.Lo[axis] + b.Hi[axis])
		switch {
		case diff < 0:
			return -1
		case diff == 0:
			return 0
		default:
			return 1
		}
	})
}

func surfaceArea(loX, loY, loZ, hiX, hiY, hiZ float32) float32 {
	dx := hiX - loX
	dy := hiY - loY
	dz := hiZ - loZ
	return 2 * (dx*dy + dy*dz + dz*dx)
}
