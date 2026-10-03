//go:build !goexperiment.simd

package bvh

import (
	"slices"

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
// invoking checkRef with the index of any scene element that could be hit.
func (t *Tree) RayTest(point, slope fvec4.T, checkRef func(ref int)) {
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
		if bvhLeafRayTest(t.Root.Lo, t.Root.Hi, point, inverseSlope) {
			checkRef(t.Root.Ref)
		}
		return
	}

	workStack := make([]*Node, 0, 32)
	workStack = append(workStack, t.Root)
	for len(workStack) != 0 {
		node := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		switch node.Type {
		case NodeTypeConcrete:
			// This node was already bounds-tested as one of its parent's
			// children, so it can be reported directly.
			checkRef(node.Ref)
		case NodeTypeComposite:
			for i := 0; i < node.NumChildren; i++ {
				if bvhChildRayTest(node, i, point, inverseSlope) {
					workStack = append(workStack, node.Children[i])
				}
			}
		}
	}
}

func (t *Tree) BuildFromElements(elements []*Node) {
	t.Root = t.assembleBVHTree(elements, 0)
}

func (t *Tree) assembleBVHTree(elements []*Node, axis int) *Node {
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

	// Recursive case.  Sort along the split axis and divide into
	// bvhBranchingFactor roughly-equal groups, each becoming one child.
	slices.SortFunc(elements, func(a, b *Node) int {
		diff := a.Lo[axis] - b.Lo[axis]
		switch {
		case diff < 0:
			return -1
		case diff == 0:
			return 0
		default:
			return 1
		}
	})

	n := len(elements)
	children := make([]*Node, 0, bvhBranchingFactor)
	for i := 0; i < bvhBranchingFactor; i++ {
		lo := i * n / bvhBranchingFactor
		hi := (i + 1) * n / bvhBranchingFactor
		if lo == hi {
			continue
		}
		children = append(children, t.assembleBVHTree(elements[lo:hi], (axis+1)%3))
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
// method (https://tavianator.com/2011/ray_box.html).  It only exists to
// handle a scene with exactly one element, where the tree's root is that
// leaf itself and so was never tested as part of a parent's child loop.
func bvhLeafRayTest(lo, hi, point, inverseSlope fvec4.T) bool {
	tx1 := (lo[0] - point[0]) * inverseSlope[0]
	tx2 := (hi[0] - point[0]) * inverseSlope[0]
	tMin := min(tx1, tx2)
	tMax := max(tx1, tx2)

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
// (https://tavianator.com/2011/ray_box.html).
func bvhChildRayTest(node *Node, i int, point, inverseSlope fvec4.T) bool {
	tx1 := (node.ChildLoX[i] - point[0]) * inverseSlope[0]
	tx2 := (node.ChildHiX[i] - point[0]) * inverseSlope[0]

	tMin := min(tx1, tx2)
	tMax := max(tx1, tx2)

	ty1 := (node.ChildLoY[i] - point[1]) * inverseSlope[1]
	ty2 := (node.ChildHiY[i] - point[1]) * inverseSlope[1]

	tMin = max(tMin, min(ty1, ty2))
	tMax = min(tMax, max(ty1, ty2))

	tz1 := (node.ChildLoZ[i] - point[2]) * inverseSlope[2]
	tz2 := (node.ChildHiZ[i] - point[2]) * inverseSlope[2]

	tMin = max(tMin, min(tz1, tz2))
	tMax = min(tMax, max(tz1, tz2))

	return tMin <= tMax
}
