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

	// Valid when Type == NodeTypeConcrete: the index of this node's
	// triangles in Tree.leaves.  A node's own bounds aren't stored in it,
	// since traversal only ever tests them as one lane of its parent's
	// ChildLoX/Y/Z and ChildHiX/Y/Z (or, for the root, Tree.rootBounds).
	Ref int

	// Valid when Type == NodeTypeComposite.  Children holds NumChildren
	// child indices into Tree.nodes (trailing slots unused).  ChildLoX/Y/Z and
	// ChildHiX/Y/Z store the same children's bounds in struct-of-arrays
	// form -- one lane per child -- so the amd64/SIMD build can test all of
	// a node's children with a handful of SIMD ops instead of one test per
	// child.  The purego build tests them with a plain scalar loop.
	NumChildren                  int
	Children                     [bvhBranchingFactor]int32
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
	// nodes holds every node of the tree, which refer to each other by
	// index.  Build lays them out depth-first, so the root is nodes[0], and
	// the tree is empty if nodes is.
	nodes []Node

	// rootBounds holds the bounds of nodes[0], the one node with no parent
	// to hold them.
	rootBounds aabb

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

// Clear removes all nodes and triangles from the tree.  The tree's storage is
// kept (with its length reset), so that rebuilding the tree doesn't
// reallocate it.
func (t *Tree) Clear() {
	t.nodes = t.nodes[:0]
	t.triangles = t.triangles[:0]
	t.elements = t.elements[:0]
	t.leaves = t.leaves[:0]
}

// newNode appends a zeroed node to t.nodes, returning its index.  It
// invalidates pointers into t.nodes.
func (t *Tree) newNode() int32 {
	t.nodes = append(t.nodes, Node{})
	return int32(len(t.nodes) - 1)
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
	if len(t.elements) == 0 {
		return
	}
	_, t.rootBounds = t.assembleBVHTree(span{first: 0, count: len(t.elements)})
}

// assembleBVHTree builds a subtree over the (non-empty) elements located by
// s, returning the index of its root in t.nodes and its bounds.  Once it has split them, it
// only reorders elements within each part, so each leaf's elements stay
// contiguous.
func (t *Tree) assembleBVHTree(s span) (int32, aabb) {
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

	// Take this node's slot before building its children, so that each
	// subtree is laid out right after its root.
	node := t.newNode()
	var children [bvhBranchingFactor]int32
	var bounds [bvhBranchingFactor]aabb
	for i, group := range groups[:numGroups] {
		children[i], bounds[i] = t.assembleBVHTree(group)
	}

	return node, t.fillComposite(node, children[:numGroups], bounds[:numGroups])
}

// fillComposite makes t.nodes[index] a composite node directly owning the
// given children (at most bvhBranchingFactor of them), with the given bounds,
// filling in the per-child struct-of-arrays bounds used to test them against
// rays.  It returns the node's own overall bounds (the union of its
// children's), for its parent to store in turn.
func (t *Tree) fillComposite(index int32, children []int32, bounds []aabb) aabb {
	node := &t.nodes[index]
	node.Type = NodeTypeComposite
	node.NumChildren = len(children)

	lo := bounds[0].Lo
	hi := bounds[0].Hi

	for i, ci := range children {
		c := &bounds[i]
		node.Children[i] = ci
		node.ChildLoX[i], node.ChildLoY[i], node.ChildLoZ[i] = c.Lo[0], c.Lo[1], c.Lo[2]
		node.ChildHiX[i], node.ChildHiY[i], node.ChildHiZ[i] = c.Hi[0], c.Hi[1], c.Hi[2]

		lo[0] = min(lo[0], c.Lo[0])
		lo[1] = min(lo[1], c.Lo[1])
		lo[2] = min(lo[2], c.Lo[2])
		hi[0] = max(hi[0], c.Hi[0])
		hi[1] = max(hi[1], c.Hi[1])
		hi[2] = max(hi[2], c.Hi[2])
	}

	return aabb{Lo: lo, Hi: hi}
}
