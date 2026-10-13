//go:build goexperiment.simd && amd64

package bvh

import (
	"math/bits"
	"simd/archsimd"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// RayTest fires a ray (origin point, direction slope) through the BVH tree,
// invoking checkRef with the index of any scene element whose bounds the ray
// overlaps for some ray parameter t in [tLo, tHi].
//
// checkRef returns a new upper bound for t -- normally the t of the closest
// hit found so far -- and the rest of the traversal skips any bounds the ray
// only enters beyond it.  Returning +Inf (or anything >= the current bound)
// leaves the bound unchanged.  Children are visited nearest-first, so a
// closest-hit query tightens the bound as early as possible.
//
// The traversal's work stack is kept in stack, which may be nil to use a
// temporary one.  The work done is added into stats, which may be nil.
func (t *Tree) RayTest(point, slope fvec4.T, tLo, tHi float32, stack *Stack, stats *Stats, checkRef func(ref int) float32) {
	t.rayTest(point, slope, tLo, tHi, stack, stats, true, checkRef)
}

// RayTestUnordered is RayTest without the nearest-first ordering: a node's
// hit children are visited in whatever order is cheapest.  That suits
// any-hit queries (e.g. shadow rays), where checkRef stops the traversal at
// the first hit by returning a bound below tLo, and sorting children by
// distance would be wasted work.
func (t *Tree) RayTestUnordered(point, slope fvec4.T, tLo, tHi float32, stack *Stack, stats *Stats, checkRef func(ref int) float32) {
	t.rayTest(point, slope, tLo, tHi, stack, stats, false, checkRef)
}

// rayTest implements RayTest (ordered set) and RayTestUnordered.
func (t *Tree) rayTest(point, slope fvec4.T, tLo, tHi float32, stack *Stack, stats *Stats, ordered bool, checkRef func(ref int) float32) {
	if t.Root == nil {
		return
	}

	// Every node pushed onto workStack below has already been bounds-tested
	// as one lane of its parent's bvhRayTest4 call. The root has no parent,
	// so if it's a leaf, it must be tested directly here instead.
	if t.Root.Type == NodeTypeConcrete {
		if bvhLeafRayTest(t.Root.Lo, t.Root.Hi, point, slope, tLo, tHi) {
			checkRef(t.Root.Ref)
		}
		return
	}

	pointX := archsimd.BroadcastFloat32x4(point[0])
	pointY := archsimd.BroadcastFloat32x4(point[1])
	pointZ := archsimd.BroadcastFloat32x4(point[2])

	invSlopeX := archsimd.BroadcastFloat32x4(1.0 / slope[0])
	invSlopeY := archsimd.BroadcastFloat32x4(1.0 / slope[1])
	invSlopeZ := archsimd.BroadcastFloat32x4(1.0 / slope[2])

	tLoV := archsimd.BroadcastFloat32x4(tLo)

	var innerVisited, leafRefs int64

	if stack == nil {
		stack = &Stack{}
	}

	// This call's entries go on top of any left by an enclosing RayTest
	// call, and are all popped by the time workStack is back down to base.
	base := len(stack.entries)
	workStack := append(stack.entries, rayTestEntry{node: t.Root, tEnter: tLo})
	for len(workStack) != base {
		entry := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		// tHi may have shrunk since this entry was pushed, putting its
		// bounds entirely beyond the closest hit found so far.
		if entry.tEnter > tHi {
			continue
		}

		node := entry.node
		switch node.Type {
		case NodeTypeConcrete:
			leafRefs++
			// This node was already bounds-tested as one lane of its
			// parent's bvhRayTest4 call, so it can be reported directly.
			//
			// checkRef may make a nested RayTest call on the same stack,
			// which pushes on top of ours (possibly growing it) and pops
			// back down to our length before returning.
			stack.entries = workStack
			tHi = min(tHi, checkRef(node.Ref))
			workStack = stack.entries
		case NodeTypeComposite:
			innerVisited++
			var tEnter [bvhBranchingFactor]float32
			hitMask := bvhRayTest4(
				&node.ChildLoX, &node.ChildLoY, &node.ChildLoZ,
				&node.ChildHiX, &node.ChildHiY, &node.ChildHiZ,
				pointX, pointY, pointZ,
				invSlopeX, invSlopeY, invSlopeZ,
				tLoV, archsimd.BroadcastFloat32x4(tHi),
				&tEnter,
			)
			// Padding lanes beyond NumChildren hold garbage; mask them off.
			hitMask &= uint8(1<<node.NumChildren) - 1

			if !ordered {
				for hitMask != 0 {
					i := bits.TrailingZeros8(hitMask)
					hitMask &= hitMask - 1
					workStack = append(workStack, rayTestEntry{node: node.Children[i], tEnter: tEnter[i]})
				}
				continue
			}

			// Order the hit children far-to-near, so that pushing them in
			// that order leaves the nearest on top of the stack.
			var order [bvhBranchingFactor]int
			n := 0
			for hitMask != 0 {
				i := bits.TrailingZeros8(hitMask)
				hitMask &= hitMask - 1

				j := n
				for j > 0 && tEnter[order[j-1]] < tEnter[i] {
					order[j] = order[j-1]
					j--
				}
				order[j] = i
				n++
			}
			for _, i := range order[:n] {
				workStack = append(workStack, rayTestEntry{node: node.Children[i], tEnter: tEnter[i]})
			}
		}
	}

	stack.entries = workStack

	if stats != nil {
		stats.InnerVisited += innerVisited
		stats.LeafRefs += leafRefs
	}
}

// bvhRayTest4 tests a ray (given as broadcast point/inverse-slope lanes) against
// up to bvhBranchingFactor sibling AABBs at once, one per lane of the
// childLo*/childHi* struct-of-arrays bounds, using the slab method
// (https://tavianator.com/2011/ray_box.html), restricted to ray parameters in
// [tLo, tHi].  It returns a bitmask with bit i set if lane i was hit, and
// stores into tEnter the ray parameter at which each lane's box is entered
// (clamped to tLo).  Lanes beyond the node's real child count hold whatever
// was last stored there and must be masked off by the caller.
func bvhRayTest4(
	childLoX, childLoY, childLoZ *[bvhBranchingFactor]float32,
	childHiX, childHiY, childHiZ *[bvhBranchingFactor]float32,
	pointX, pointY, pointZ archsimd.Float32x4,
	invSlopeX, invSlopeY, invSlopeZ archsimd.Float32x4,
	tLo, tHi archsimd.Float32x4,
	tEnter *[bvhBranchingFactor]float32,
) uint8 {
	loX := archsimd.LoadFloat32x4(childLoX)
	loY := archsimd.LoadFloat32x4(childLoY)
	loZ := archsimd.LoadFloat32x4(childLoZ)
	hiX := archsimd.LoadFloat32x4(childHiX)
	hiY := archsimd.LoadFloat32x4(childHiY)
	hiZ := archsimd.LoadFloat32x4(childHiZ)

	txLo := loX.Sub(pointX).Mul(invSlopeX)
	txHi := hiX.Sub(pointX).Mul(invSlopeX)
	tyLo := loY.Sub(pointY).Mul(invSlopeY)
	tyHi := hiY.Sub(pointY).Mul(invSlopeY)
	tzLo := loZ.Sub(pointZ).Mul(invSlopeZ)
	tzHi := hiZ.Sub(pointZ).Mul(invSlopeZ)

	tMin := txLo.Min(txHi)
	tMax := txLo.Max(txHi)
	tMin = tMin.Max(tyLo.Min(tyHi))
	tMax = tMax.Min(tyLo.Max(tyHi))
	tMin = tMin.Max(tzLo.Min(tzHi))
	tMax = tMax.Min(tzLo.Max(tzHi))

	tMin = tMin.Max(tLo)
	tMax = tMax.Min(tHi)

	tMin.Store(tEnter)
	return tMin.LessEqual(tMax).ToBits()
}

// bvhLeafRayTest tests a ray against a single AABB directly, using the slab
// method (https://tavianator.com/2011/ray_box.html), restricted to ray
// parameters in [tLo, tHi].  It only exists to handle a scene with exactly
// one element, where the tree's root is that leaf itself and so was never
// tested as part of a parent's bvhRayTest4 call; this is not a hot path, so
// it's plain scalar code rather than SIMD.
func bvhLeafRayTest(lo, hi fvec4.T, point, slope fvec4.T, tLo, tHi float32) bool {
	invSlope := fvec4.T{1.0 / slope[0], 1.0 / slope[1], 1.0 / slope[2], 0}

	tx1 := (lo[0] - point[0]) * invSlope[0]
	tx2 := (hi[0] - point[0]) * invSlope[0]
	tMin := max(tLo, min(tx1, tx2))
	tMax := min(tHi, max(tx1, tx2))

	ty1 := (lo[1] - point[1]) * invSlope[1]
	ty2 := (hi[1] - point[1]) * invSlope[1]
	tMin = max(tMin, min(ty1, ty2))
	tMax = min(tMax, max(ty1, ty2))

	tz1 := (lo[2] - point[2]) * invSlope[2]
	tz2 := (hi[2] - point[2]) * invSlope[2]
	tMin = max(tMin, min(tz1, tz2))
	tMax = min(tMax, max(tz1, tz2))

	return tMin <= tMax
}
