//go:build !goexperiment.simd || !amd64

package meshbvh

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// rayTest fires a ray (origin point, direction slope) through the tree,
// invoking checkLeaf with the Ref of every leaf whose bounds the ray overlaps
// for some ray parameter t in [tLo, tHi].
//
// checkLeaf returns a new upper bound for t -- normally the t of the closest
// hit found so far -- and the rest of the traversal skips any bounds the ray
// only enters beyond it.  Returning +Inf (or anything >= the current bound)
// leaves the bound unchanged, and returning anything below tLo stops the
// traversal.  If ordered is set, children are visited nearest-first, so a
// closest-hit query tightens the bound as early as possible; otherwise
// they're visited in whatever order is cheapest, which suits any-hit queries.
//
// The traversal's work stack is kept in stack, which may be nil to use a
// temporary one.  The work done is added into stats, which may be nil.
func (t *Tree) rayTest(point, slope fvec4.T, tLo, tHi float32, stack *Stack, stats *Stats, ordered bool, checkLeaf func(leaf int) float32) {
	if len(t.nodes) == 0 {
		return
	}

	nodes := t.nodes
	root := &nodes[0]

	inverseSlope := fvec4.T{
		1.0 / slope[0],
		1.0 / slope[1],
		1.0 / slope[2],
	}

	// Every node pushed onto workStack below has already been bounds-tested
	// as one of its parent's children. The root has no parent, so if it's a
	// leaf, it must be tested directly here instead.
	if root.Type == NodeTypeConcrete {
		if bvhLeafRayTest(t.rootBounds.Lo, t.rootBounds.Hi, point, inverseSlope, tLo, tHi) {
			checkLeaf(root.Ref)
		}
		return
	}

	var innerVisited, leafRefs int64

	if stack == nil {
		stack = &Stack{}
	}

	// This call's entries go on top of any left by an enclosing RayTest
	// call, and are all popped by the time workStack is back down to base.
	base := len(stack.entries)
	workStack := append(stack.entries, rayTestEntry{node: 0, tEnter: tLo})
	for len(workStack) != base {
		entry := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		// tHi may have shrunk since this entry was pushed, putting its
		// bounds entirely beyond the closest hit found so far.
		if entry.tEnter > tHi {
			continue
		}

		node := &nodes[entry.node]
		switch node.Type {
		case NodeTypeConcrete:
			leafRefs++
			// This node was already bounds-tested as one of its parent's
			// children, so it can be reported directly.
			tHi = min(tHi, checkLeaf(node.Ref))
		case NodeTypeComposite:
			innerVisited++
			// Collect the hit children, ordered far-to-near unless
			// unordered, so that pushing them in that order leaves the
			// nearest on top of the stack.
			var tEnter [bvhBranchingFactor]float32
			var order [bvhBranchingFactor]int
			n := 0
			for i := 0; i < node.NumChildren; i++ {
				var ok bool
				tEnter[i], ok = bvhChildRayTest(node, i, point, inverseSlope, tLo, tHi)
				if !ok {
					continue
				}

				if !ordered {
					workStack = append(workStack, rayTestEntry{node: node.Children[i], tEnter: tEnter[i]})
					continue
				}

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

// bvhLeafRayTest tests a ray against a single AABB directly, using the slab
// method (https://tavianator.com/2011/ray_box.html), restricted to ray
// parameters in [tLo, tHi].  It only exists to handle a tree with at most
// LeafSize elements, where the tree's root is a leaf and so was never
// tested as part of a parent's child loop.
func bvhLeafRayTest(lo, hi, point, inverseSlope fvec4.T, tLo, tHi float32) bool {
	tx1 := (lo[0] - point[0]) * inverseSlope[0]
	tx2 := (hi[0] - point[0]) * inverseSlope[0]
	tMin := max(tLo, min(tx1, tx2))
	tMax := min(tHi, max(tx1, tx2))

	ty1 := (lo[1] - point[1]) * inverseSlope[1]
	ty2 := (hi[1] - point[1]) * inverseSlope[1]
	tMin = max(tMin, min(ty1, ty2))
	tMax = min(tMax, max(ty1, ty2))

	tz1 := (lo[2] - point[2]) * inverseSlope[2]
	tz2 := (hi[2] - point[2]) * inverseSlope[2]
	tMin = max(tMin, min(tz1, tz2))
	tMax = min(tMax, max(tz1, tz2))

	return tMin <= tMax
}

// bvhChildRayTest tests a ray against child i of node, using the slab method
// (https://tavianator.com/2011/ray_box.html), restricted to ray parameters in
// [tLo, tHi].  On a hit, it also returns the ray parameter at which the
// child's box is entered (clamped to tLo).
func bvhChildRayTest(node *Node, i int, point, inverseSlope fvec4.T, tLo, tHi float32) (float32, bool) {
	tx1 := (node.ChildLoX[i] - point[0]) * inverseSlope[0]
	tx2 := (node.ChildHiX[i] - point[0]) * inverseSlope[0]

	tMin := max(tLo, min(tx1, tx2))
	tMax := min(tHi, max(tx1, tx2))

	ty1 := (node.ChildLoY[i] - point[1]) * inverseSlope[1]
	ty2 := (node.ChildHiY[i] - point[1]) * inverseSlope[1]

	tMin = max(tMin, min(ty1, ty2))
	tMax = min(tMax, max(ty1, ty2))

	tz1 := (node.ChildLoZ[i] - point[2]) * inverseSlope[2]
	tz2 := (node.ChildHiZ[i] - point[2]) * inverseSlope[2]

	tMin = max(tMin, min(tz1, tz2))
	tMax = min(tMax, max(tz1, tz2))

	return tMin, tMin <= tMax
}
