package bvh

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// FrustumTest invokes checkRef with the index of every scene element whose
// bounds aren't entirely outside one of planes.  It's conservative: an
// element's bounds can be reported while lying outside the intersection of
// the planes, for example near the corners of a frustum.
//
// Plane i is (n, d) packed as {n[0], n[1], n[2], d}, and holds the points p
// with n.p + d >= 0.  The first plane also orders the traversal: children are
// visited nearest-first along its normal, so that if it's a camera's near
// plane, elements tend to be reported front-to-back.
//
// The traversal's work stack is kept in stack, which may be nil to use a
// temporary one; as with RayTest, checkRef may make nested calls on the same
// stack.  The work done is added into stats, which may be nil.
func (t *Tree) FrustumTest(planes []fvec4.T, stack *Stack, stats *Stats, checkRef func(ref int)) {
	if t.Root == nil || len(planes) == 0 {
		return
	}

	// As in RayTest, every node pushed onto the work stack has already been
	// tested as one of its parent's children, except the root.
	if t.Root.Type == NodeTypeConcrete {
		lo, hi := t.Root.Lo, t.Root.Hi
		for _, p := range planes {
			if boxPlaneMax(p, lo[0], lo[1], lo[2], hi[0], hi[1], hi[2]) < 0 {
				return
			}
		}
		checkRef(t.Root.Ref)
		return
	}

	var innerVisited, leafRefs int64

	if stack == nil {
		stack = &Stack{}
	}

	near := planes[0]
	base := len(stack.entries)
	workStack := append(stack.entries, rayTestEntry{node: t.Root})
	for len(workStack) != base {
		entry := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		node := entry.node
		switch node.Type {
		case NodeTypeConcrete:
			leafRefs++
			stack.entries = workStack
			checkRef(node.Ref)
			workStack = stack.entries
		case NodeTypeComposite:
			innerVisited++
			// Collect the children overlapping every plane, ordered
			// far-to-near along the first plane, so that pushing them in
			// that order leaves the nearest on top of the stack.
			var key [bvhBranchingFactor]float32
			var order [bvhBranchingFactor]int
			n := 0
		children:
			for i := 0; i < node.NumChildren; i++ {
				loX, loY, loZ := node.ChildLoX[i], node.ChildLoY[i], node.ChildLoZ[i]
				hiX, hiY, hiZ := node.ChildHiX[i], node.ChildHiY[i], node.ChildHiZ[i]
				for _, p := range planes {
					if boxPlaneMax(p, loX, loY, loZ, hiX, hiY, hiZ) < 0 {
						continue children
					}
				}

				// The nearest corner along the first plane's normal.
				key[i] = -boxPlaneMax(fvec4.T{-near[0], -near[1], -near[2], -near[3]}, loX, loY, loZ, hiX, hiY, hiZ)

				j := n
				for j > 0 && key[order[j-1]] < key[i] {
					order[j] = order[j-1]
					j--
				}
				order[j] = i
				n++
			}
			for _, i := range order[:n] {
				workStack = append(workStack, rayTestEntry{node: node.Children[i], tEnter: key[i]})
			}
		}
	}

	stack.entries = workStack

	if stats != nil {
		stats.InnerVisited += innerVisited
		stats.LeafRefs += leafRefs
	}
}

// boxPlaneMax returns the largest value of n.p + d over the points p of the
// box [lo, hi], for plane p = {n[0], n[1], n[2], d}.  The box is entirely
// outside the plane if it's negative.
func boxPlaneMax(p fvec4.T, loX, loY, loZ, hiX, hiY, hiZ float32) float32 {
	return p[0]*pick(p[0], loX, hiX) + p[1]*pick(p[1], loY, hiY) + p[2]*pick(p[2], loZ, hiZ) + p[3]
}

// pick returns hi if n is non-negative, and lo otherwise.
func pick(n, lo, hi float32) float32 {
	if n >= 0 {
		return hi
	}
	return lo
}
