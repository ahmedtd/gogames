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

// aabb is an axis-aligned bounding box.  Only the first three lanes of Lo
// and Hi are used.
type aabb struct {
	Lo, Hi fvec4.T
}

// rawElement is a triangle to build a tree over: its bounds, and its index
// into Tree.triangles.
type rawElement struct {
	Bounds aabb
	Ref    int
}

type NodeType int

const (
	NodeTypeConcrete NodeType = iota
	NodeTypeComposite
)

type Node struct {
	Type NodeType

	// Valid when Type == NodeTypeConcrete: this node's own bounds, and the
	// index of its triangles in Tree.leaves.  Also valid (as the union of children's
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

// LeafSize is the most triangles Build gathers into each leaf, matching the
// SIMD register width used by leafTris.rayIntersect (Float32x4 -- 4 lanes).
//
// 8 lanes (Float32x8) was tried, and rendered slower: leaves of 8 triangles
// bound them so loosely that a ray tested over twice as many triangles, and
// the rasterizer drew a fifth more.
const LeafSize = 4

// span locates a run of elements: Tree.elements[first:first+count].
type span struct {
	first, count int
}

type Tree struct {
	Root *Node

	FreeList                  []*Node
	AllocationsSinceLastClear int

	// triangles holds the triangles added by AddTriangle, indexed by ref.
	triangles []Triangle

	// elements holds the bounds of each triangle, for Build to sort and
	// split.  Build reorders it, leaving each leaf's triangles contiguous.
	elements []rawElement

	// leaves[i] holds the triangles of the leaf with Ref i.
	leaves []leafTris

	// Scratch space for splitSAH, kept to avoid reallocating it on every
	// build.
	sahScratch []float32
}

// Clear moves all nodes from the tree to the free list, and removes all
// triangles.  The tree's other storage is kept (with its length reset), so
// that rebuilding the tree doesn't reallocate it.
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
	t.triangles = t.triangles[:0]
	t.elements = t.elements[:0]
	t.leaves = t.leaves[:0]
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

// AddTriangle adds a triangle for the next Build to include.  Its ref, as
// reported by queries, is the number of triangles added before it since the
// last Clear.
func (t *Tree) AddTriangle(tri Triangle) {
	t.elements = append(t.elements, rawElement{Bounds: tri.bounds(), Ref: len(t.triangles)})
	t.triangles = append(t.triangles, tri)
}

// Build builds the tree over the triangles added since the last Clear.
func (t *Tree) Build() {
	t.Root = t.assembleBVHTree(span{first: 0, count: len(t.elements)})
}

// assembleBVHTree builds a subtree over the elements located by s.  Once it
// has split them, it only reorders elements within each part, so each leaf's
// elements stay contiguous.
func (t *Tree) assembleBVHTree(s span) *Node {
	// Base case.
	if s.count == 0 {
		return nil
	}

	// Base case.  Few enough elements left to gather into one leaf.
	if s.count <= LeafSize {
		return t.makeLeaf(s)
	}

	// Recursive case.  Split the elements in two using the surface area
	// heuristic, then split each half in two again the same way, and make
	// each of the resulting (up to) four groups one child.  This assumes
	// bvhBranchingFactor == 4.
	mid := t.splitSAH(t.elements[s.first : s.first+s.count])
	halves := [2]span{
		{first: s.first, count: mid},
		{first: s.first + mid, count: s.count - mid},
	}
	var groups [bvhBranchingFactor]span
	numGroups := 0
	for _, half := range halves {
		if half.count <= LeafSize {
			groups[numGroups] = half
			numGroups++
			continue
		}
		quarterMid := t.splitSAH(t.elements[half.first : half.first+half.count])
		groups[numGroups] = span{first: half.first, count: quarterMid}
		groups[numGroups+1] = span{first: half.first + quarterMid, count: half.count - quarterMid}
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
