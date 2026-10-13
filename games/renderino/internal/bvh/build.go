package bvh

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// AABB is an axis-aligned bounding box.  Only the first three lanes of Lo
// and Hi are used.
type AABB struct {
	Lo, Hi fvec4.T
}

// RawElement is a scene element to build a tree over: its bounds, and the
// ref reported for it by queries such as RayTest.
type RawElement struct {
	Bounds AABB
	Ref    int
}

type Tree struct {
	Root *Node

	FreeList                  []*Node
	AllocationsSinceLastClear int

	// Elements holds the elements added by AddElement, which the next Build
	// will build the tree over.  It's kept across Clear (with its length
	// reset) so that rebuilding the tree every frame doesn't reallocate it.
	// Build reorders it.
	Elements []RawElement

	// LeafSize, if more than 1, makes Build gather up to LeafSize elements
	// into each leaf, rather than giving every element a leaf of its own.  A
	// leaf's Ref is then an index into LeafGroups, so that a caller can lay
	// out each group's elements together (e.g. to test them all at once with
	// SIMD).
	LeafSize int

	// LeafGroups[g] holds the refs of the elements gathered into the leaf
	// with Ref g.  Only built when LeafSize > 1.
	LeafGroups [][]int

	// Scratch space for splitSAH, kept to avoid reallocating it on every
	// build.
	sahScratch []float32
}

// Clear moves all nodes from the tree to the free list, and removes all
// elements.
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
	t.Elements = t.Elements[:0]
	t.LeafGroups = nil
}

func (t *Tree) getFreeNode() *Node {
	if len(t.FreeList) == 0 {
		t.AllocationsSinceLastClear++
		return &Node{}
	}

	picked := t.FreeList[len(t.FreeList)-1]
	t.FreeList = t.FreeList[:len(t.FreeList)-1]

	return picked
}

// AddElement adds an element for the next Build to include.
func (t *Tree) AddElement(bounds AABB, ref int) {
	t.Elements = append(t.Elements, RawElement{Bounds: bounds, Ref: ref})
}

// Build builds the tree over the elements added since the last Clear.
func (t *Tree) Build() {
	t.Root = t.assembleBVHTree(t.Elements)
}

func (t *Tree) assembleBVHTree(elements []RawElement) *Node {
	// Base case.
	if len(elements) == 0 {
		return nil
	}

	// Base case.  Few enough elements left to gather into one leaf.
	if t.LeafSize > 1 && len(elements) <= t.LeafSize {
		return t.makeLeafGroup(elements)
	}

	// Base case.  A single element just becomes the root -- this will be
	// absorbed as a child the next level up, or returned directly if it's
	// the only element in the whole tree.
	if len(elements) == 1 {
		return t.makeLeaf(elements[0])
	}

	// Base case.  Few enough elements left to all become direct children of
	// one composite node.
	if len(elements) <= bvhBranchingFactor {
		var children [bvhBranchingFactor]*Node
		for i, e := range elements {
			children[i] = t.makeLeaf(e)
		}
		return t.makeComposite(children[:len(elements)])
	}

	// Recursive case.  Split the elements in two using the surface area
	// heuristic, then split each half in two again the same way, and make
	// each of the resulting (up to) four groups one child.  This assumes
	// bvhBranchingFactor == 4.
	mid := t.splitSAH(elements)
	var groups [bvhBranchingFactor][]RawElement
	numGroups := 0
	for _, half := range [2][]RawElement{elements[:mid], elements[mid:]} {
		if len(half) <= max(1, t.LeafSize) {
			groups[numGroups] = half
			numGroups++
			continue
		}
		quarterMid := t.splitSAH(half)
		groups[numGroups] = half[:quarterMid]
		groups[numGroups+1] = half[quarterMid:]
		numGroups += 2
	}

	var children [bvhBranchingFactor]*Node
	for i, group := range groups[:numGroups] {
		children[i] = t.assembleBVHTree(group)
	}

	return t.makeComposite(children[:numGroups])
}

// makeLeaf builds a leaf node for a single element.
func (t *Tree) makeLeaf(e RawElement) *Node {
	node := t.getFreeNode()
	node.Type = NodeTypeConcrete
	node.Lo, node.Hi = e.Bounds.Lo, e.Bounds.Hi
	node.Ref = e.Ref
	return node
}

// makeComposite builds a composite node directly owning the given children
// (at most bvhBranchingFactor of them), filling in both the per-child
// struct-of-arrays bounds used to test them against rays and the node's own
// overall bounds (the union of its children's), the latter so that this
// node's bounds are available if it's in turn absorbed as someone else's
// child.
func (t *Tree) makeComposite(children []*Node) *Node {
	node := t.getFreeNode()
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
