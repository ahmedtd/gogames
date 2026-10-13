package meshbvh

import (
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// bvhBranchingFactor is the number of children a composite BVH node can
// have.  The amd64/SIMD build tests all of a node's children against a ray in
// one SIMD pass over a [bvhBranchingFactor]float32 lane per bound, so this is
// tied to the SIMD register width used by bvhRayTest4 (Float32x4 -- 4 lanes).
// The purego build uses the same value so the tree shape (and thus traversal
// depth) is comparable between the two implementations.
const bvhBranchingFactor = 4

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
	// form -- one lane per child -- so the amd64/SIMD build can test all of
	// a node's children with a handful of SIMD ops instead of one test per
	// child.  The purego build tests them with a plain scalar loop.
	NumChildren                  int
	Children                     [bvhBranchingFactor]*Node
	ChildLoX, ChildLoY, ChildLoZ [bvhBranchingFactor]float32
	ChildHiX, ChildHiY, ChildHiZ [bvhBranchingFactor]float32
}

// LeafSize is the most elements Build gathers into each leaf, matching the
// SIMD register width (Float32x4 -- 4 lanes) a caller tests a leaf's elements
// with.
const LeafSize = 4

// LeafGroup locates the elements gathered into one leaf: they're
// Elements[First:First+Count] of the Tree that built it.
type LeafGroup struct {
	First, Count int
}

type Tree struct {
	Root *Node

	FreeList                  []*Node
	AllocationsSinceLastClear int

	// Elements holds the elements added by AddElement, which the next Build
	// will build the tree over.  It's kept across Clear (with its length
	// reset) so that rebuilding the tree every frame doesn't reallocate it.
	// Build reorders it, leaving each leaf group's elements contiguous.
	Elements []RawElement

	// LeafGroups[g] locates the elements gathered into the leaf with Ref g.
	// A leaf's Ref is an index into LeafGroups, so that a caller can lay out
	// each group's elements together (e.g. to test them all at once with
	// SIMD).
	LeafGroups []LeafGroup

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
	t.LeafGroups = t.LeafGroups[:0]
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
	t.Root = t.assembleBVHTree(LeafGroup{First: 0, Count: len(t.Elements)})
}

// LeafElements returns the elements gathered into the leaf with Ref g.
func (t *Tree) LeafElements(g int) []RawElement {
	group := t.LeafGroups[g]
	return t.Elements[group.First : group.First+group.Count]
}

// assembleBVHTree builds a subtree over the elements located by span.  Once
// it has split them, it only reorders elements within each part, so each
// leaf group stays contiguous.
func (t *Tree) assembleBVHTree(span LeafGroup) *Node {
	// Base case.
	if span.Count == 0 {
		return nil
	}

	// Base case.  Few enough elements left to gather into one leaf.
	if span.Count <= LeafSize {
		return t.makeLeafGroup(span)
	}

	// Recursive case.  Split the elements in two using the surface area
	// heuristic, then split each half in two again the same way, and make
	// each of the resulting (up to) four groups one child.  This assumes
	// bvhBranchingFactor == 4.
	mid := t.splitSAH(t.Elements[span.First : span.First+span.Count])
	halves := [2]LeafGroup{
		{First: span.First, Count: mid},
		{First: span.First + mid, Count: span.Count - mid},
	}
	var groups [bvhBranchingFactor]LeafGroup
	numGroups := 0
	for _, half := range halves {
		if half.Count <= LeafSize {
			groups[numGroups] = half
			numGroups++
			continue
		}
		quarterMid := t.splitSAH(t.Elements[half.First : half.First+half.Count])
		groups[numGroups] = LeafGroup{First: half.First, Count: quarterMid}
		groups[numGroups+1] = LeafGroup{First: half.First + quarterMid, Count: half.Count - quarterMid}
		numGroups += 2
	}

	var children [bvhBranchingFactor]*Node
	for i, group := range groups[:numGroups] {
		children[i] = t.assembleBVHTree(group)
	}

	return t.makeComposite(children[:numGroups])
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
