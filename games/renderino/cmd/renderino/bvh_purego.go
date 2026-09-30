//go:build !goexperiment.simd

package main

import (
	"slices"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

type BVHTree struct {
	Root *BVHNode

	FreeList                  []*BVHNode
	AllocationsSinceLastClear int
}

// Clear moves all nodes from the tree to the free list.
func (t *BVHTree) Clear() {
	//freeListLenBeforeClear := len(t.FreeList)

	workStack := []*BVHNode{t.Root}
	for len(workStack) != 0 {
		cur := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		if cur == nil {
			continue
		}

		workStack = append(workStack, cur.LoChild)
		workStack = append(workStack, cur.HiChild)
		*cur = BVHNode{}

		t.FreeList = append(t.FreeList, cur)
	}

	//log.Printf("allocationsSinceLastClear=%d freeListLenBefore=%d freeListLenAfter=%d", t.AllocationsSinceLastClear, freeListLenBeforeClear, len(t.FreeList))
	t.AllocationsSinceLastClear = 0

	t.Root = nil
}

func (t *BVHTree) GetFreeNode() *BVHNode {
	if len(t.FreeList) == 0 {
		t.AllocationsSinceLastClear++
		return &BVHNode{}
	}

	picked := t.FreeList[len(t.FreeList)-1]
	t.FreeList = t.FreeList[:len(t.FreeList)-1]

	return picked
}

// RayTest fires a ray through the BVH tree, invoking checkSceneElement with the
// index of any scene element that could be hit.
func (t *BVHTree) RayTest(ray Ray, checkRef func(ref int)) {
	inverseSlope := fvec4.T{
		1.0 / ray.Slope[0],
		1.0 / ray.Slope[1],
		1.0 / ray.Slope[2],
	}

	workStack := make([]*BVHNode, 0, 32)
	workStack = append(workStack, t.Root)
	for len(workStack) != 0 {
		node := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		if node == nil {
			continue
		}

		// If we didn't hit this node, we don't need to look at it any further.
		if !node.RayTest(ray.Point, inverseSlope) {
			continue
		}

		switch node.Type {
		case BVHNodeTypeConcrete:
			checkRef(node.Ref)
		case BVHNodeTypeComposite:
			workStack = append(workStack, node.LoChild, node.HiChild)
		}
	}
}

type BVHNodeType int

const (
	BVHNodeTypeConcrete BVHNodeType = iota
	BVHNodeTypeComposite
)

type BVHNode struct {
	Type BVHNodeType
	Lo   fvec4.T
	Hi   fvec4.T

	Ref int

	LoChild *BVHNode
	HiChild *BVHNode
}

func (t *BVHTree) BuildFromElements(elements []*BVHNode) {
	t.Root = t.assembleBVHTree(elements, 0)
}

func (t *BVHTree) assembleBVHTree(elements []*BVHNode, axis int) *BVHNode {
	// Base case.
	if len(elements) == 0 {
		return nil
	}

	// Base case.  Make root a concrete node that holds only this elements.
	if len(elements) == 1 {
		if elements[0].Type != BVHNodeTypeConcrete {
			panic("composite BVH node passed to assembleBVHTree")
		}
		return elements[0]
	}

	// Recursive case.  Pick a reasonable split and divide elements into a high
	// and low child of the current node.

	// Sort all elements according to the split axis.
	slices.SortFunc[[]*BVHNode, *BVHNode](elements, func(a, b *BVHNode) int {
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

	// Split in the middle of the list.
	splitIndex := len(elements) / 2
	loChild := t.assembleBVHTree(elements[0:splitIndex], (axis+1)%3)
	hiChild := t.assembleBVHTree(elements[splitIndex:], (axis+1)%3)

	boundsLo := fvec4.T{}
	boundsLo[0] = min(loChild.Lo[0], hiChild.Lo[0])
	boundsLo[1] = min(loChild.Lo[1], hiChild.Lo[1])
	boundsLo[2] = min(loChild.Lo[2], hiChild.Lo[2])

	boundsHi := fvec4.T{}
	boundsHi[0] = max(loChild.Hi[0], hiChild.Hi[0])
	boundsHi[1] = max(loChild.Hi[1], hiChild.Hi[1])
	boundsHi[2] = max(loChild.Hi[2], hiChild.Hi[2])

	node := t.GetFreeNode()
	node.Type = BVHNodeTypeComposite
	node.Lo = boundsLo
	node.Hi = boundsHi
	node.LoChild = loChild
	node.HiChild = hiChild

	return node
}

func (b *BVHNode) Reset(nodeType BVHNodeType, lo, hi fvec4.T, ref int) {
	b.Type = nodeType
	b.Lo = lo
	b.Hi = hi
	b.Ref = ref
}

func (b *BVHNode) RayTest(point, inverseSlope fvec4.T) bool {
	// https://tavianator.com/2011/ray_box.html

	tx1 := (b.Lo[0] - point[0]) * inverseSlope[0]
	tx2 := (b.Hi[0] - point[0]) * inverseSlope[0]

	tMin := min(tx1, tx2)
	tMax := max(tx1, tx2)

	ty1 := (b.Lo[1] - point[1]) * inverseSlope[1]
	ty2 := (b.Hi[1] - point[1]) * inverseSlope[1]

	tMin = max(tMin, min(ty1, ty2))
	tMax = min(tMax, max(ty1, ty2))

	tz1 := (b.Lo[2] - point[2]) * inverseSlope[2]
	tz2 := (b.Hi[2] - point[2]) * inverseSlope[2]

	tMin = max(tMin, min(tz1, tz2))
	tMax = min(tMax, max(tz1, tz2))

	return tMin <= tMax
}
