//go:build !goexperiment.simd

package bvh

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// bvhBranchingFactor is the number of children a composite BVH node can
// have.  Matches the amd64/SIMD build's branching factor so the tree shape
// (and thus traversal depth) is comparable between the two implementations.
const bvhBranchingFactor = 4

type Tree struct {
	Root *Node

	FreeList                  []*Node
	AllocationsSinceLastClear int

	// Scratch space for splitSAH, kept to avoid reallocating it on every
	// build.
	sahScratch []float32
}

// Clear moves all nodes from the tree to the free list.
func (t *Tree) Clear() {
	workStack := []*Node{t.Root}
	for len(workStack) != 0 {
		cur := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		if cur == nil {
			continue
		}

		for i := 0; i < cur.NumChildren; i++ {
			workStack = append(workStack, cur.Children[i])
		}
		*cur = Node{}

		t.FreeList = append(t.FreeList, cur)
	}

	t.AllocationsSinceLastClear = 0

	t.Root = nil
}

func (t *Tree) GetFreeNode() *Node {
	if len(t.FreeList) == 0 {
		t.AllocationsSinceLastClear++
		return &Node{}
	}

	picked := t.FreeList[len(t.FreeList)-1]
	t.FreeList = t.FreeList[:len(t.FreeList)-1]

	return picked
}

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
	if t.Root == nil {
		return
	}

	inverseSlope := fvec4.T{
		1.0 / slope[0],
		1.0 / slope[1],
		1.0 / slope[2],
	}

	// Every node pushed onto workStack below has already been bounds-tested
	// as one of its parent's children. The root has no parent, so if it's a
	// leaf, it must be tested directly here instead.
	if t.Root.Type == NodeTypeConcrete {
		if bvhLeafRayTest(t.Root.Lo, t.Root.Hi, point, inverseSlope, tLo, tHi) {
			checkRef(t.Root.Ref)
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
			// This node was already bounds-tested as one of its parent's
			// children, so it can be reported directly.
			//
			// checkRef may make a nested RayTest call on the same stack,
			// which pushes on top of ours (possibly growing it) and pops
			// back down to our length before returning.
			stack.entries = workStack
			tHi = min(tHi, checkRef(node.Ref))
			workStack = stack.entries
		case NodeTypeComposite:
			innerVisited++
			// Collect the hit children ordered far-to-near, so that pushing
			// them in that order leaves the nearest on top of the stack.
			var tEnter [bvhBranchingFactor]float32
			var order [bvhBranchingFactor]int
			n := 0
			for i := 0; i < node.NumChildren; i++ {
				var ok bool
				tEnter[i], ok = bvhChildRayTest(node, i, point, inverseSlope, tLo, tHi)
				if !ok {
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

func (t *Tree) BuildFromElements(elements []*Node) {
	t.Root = t.assembleBVHTree(elements)
}

func (t *Tree) assembleBVHTree(elements []*Node) *Node {
	// Base case.
	if len(elements) == 0 {
		return nil
	}

	// Base case.  A single element just becomes the root -- this will be
	// absorbed as a child the next level up, or returned directly if it's
	// the only element in the whole tree.
	if len(elements) == 1 {
		return elements[0]
	}

	// Base case.  Few enough elements left to all become direct children of
	// one composite node.
	if len(elements) <= bvhBranchingFactor {
		return t.makeComposite(elements)
	}

	// Recursive case.  Split the elements in two using the surface area
	// heuristic, then split each half in two again the same way, and make
	// each of the resulting (up to) four groups one child.  This assumes
	// bvhBranchingFactor == 4.
	mid := t.splitSAH(elements)
	var groups [bvhBranchingFactor][]*Node
	numGroups := 0
	for _, half := range [2][]*Node{elements[:mid], elements[mid:]} {
		if len(half) == 1 {
			groups[numGroups] = half
			numGroups++
			continue
		}
		quarterMid := t.splitSAH(half)
		groups[numGroups] = half[:quarterMid]
		groups[numGroups+1] = half[quarterMid:]
		numGroups += 2
	}

	children := make([]*Node, numGroups)
	for i, group := range groups[:numGroups] {
		children[i] = t.assembleBVHTree(group)
	}

	return t.makeComposite(children)
}

// makeComposite builds a composite node directly owning the given children
// (at most bvhBranchingFactor of them), filling in both the per-child
// struct-of-arrays bounds used by bvhChildRayTest and the node's own overall
// bounds (the union of its children's), the latter so that this node's
// bounds are available if it's in turn absorbed as someone else's child.
func (t *Tree) makeComposite(children []*Node) *Node {
	node := t.GetFreeNode()
	node.Type = NodeTypeComposite
	node.NumChildren = len(children)

	lo := children[0].Lo
	hi := children[0].Hi

	for i, c := range children {
		node.Children[i] = c
		node.ChildLoX[i], node.ChildLoY[i], node.ChildLoZ[i] = c.Lo[0], c.Lo[1], c.Lo[2]
		node.ChildHiX[i], node.ChildHiY[i], node.ChildHiZ[i] = c.Hi[0], c.Hi[1], c.Hi[2]

		lo[0] = min(lo[0], c.Lo[0])
		lo[1] = min(lo[1], c.Lo[1])
		lo[2] = min(lo[2], c.Lo[2])
		hi[0] = max(hi[0], c.Hi[0])
		hi[1] = max(hi[1], c.Hi[1])
		hi[2] = max(hi[2], c.Hi[2])
	}

	node.Lo = lo
	node.Hi = hi

	return node
}

type NodeType int

const (
	NodeTypeConcrete NodeType = iota
	NodeTypeComposite
)

type Node struct {
	Type NodeType

	// Valid when Type == NodeTypeConcrete: this node's own bounds and
	// scene-element reference.  Also valid (as the union of children's
	// bounds) when Type == NodeTypeComposite, since this node might in
	// turn be some other node's child.
	Lo  fvec4.T
	Hi  fvec4.T
	Ref int

	// Valid when Type == NodeTypeComposite.  Children holds NumChildren
	// child pointers (trailing slots unused).  ChildLoX/Y/Z and
	// ChildHiX/Y/Z store the same children's bounds in struct-of-arrays
	// form -- one lane per child -- matching the amd64/SIMD build's layout,
	// even though this build tests them with a plain scalar loop.
	NumChildren                  int
	Children                     [bvhBranchingFactor]*Node
	ChildLoX, ChildLoY, ChildLoZ [bvhBranchingFactor]float32
	ChildHiX, ChildHiY, ChildHiZ [bvhBranchingFactor]float32
}

func (b *Node) Reset(nodeType NodeType, lo, hi fvec4.T, ref int) {
	b.Type = nodeType
	b.Lo = lo
	b.Hi = hi
	b.Ref = ref
}

// bvhLeafRayTest tests a ray against a single AABB directly, using the slab
// method (https://tavianator.com/2011/ray_box.html), restricted to ray
// parameters in [tLo, tHi].  It only exists to handle a scene with exactly
// one element, where the tree's root is that leaf itself and so was never
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
