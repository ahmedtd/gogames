//go:build goexperiment.simd && amd64

package main

import (
	"simd/archsimd"
	"slices"

	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
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
	pointV := archsimd.LoadFloat32x4((*[4]float32)(&ray.Point))
	slopeV := archsimd.LoadFloat32x4((*[4]float32)(&ray.Slope))
	inverseSlopeV := slopeV.Reciprocal().SetElem(3, 0)

	workStack := make([]*BVHNode, 0, 32)
	workStack = append(workStack, t.Root)
	for len(workStack) != 0 {
		node := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		if node == nil {
			continue
		}

		// If we didn't hit this node, we don't need to look at it any further.
		if !node.RayTest(pointV, inverseSlopeV) {
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

func (t *BVHTree) BuildFromElements(elements []*BVHNode) {
	t.Root = t.assembleBVHTree(elements, 0)
}

func (t *BVHTree) assembleBVHTree(elements []*BVHNode, axis uint8) *BVHNode {
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
		diff := a.Lo.GetElem(axis) - b.Lo.GetElem(axis)
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

	boundsLo := [4]float32{}
	boundsLo[0] = min(loChild.Lo.GetElem(0), hiChild.Lo.GetElem(0))
	boundsLo[1] = min(loChild.Lo.GetElem(1), hiChild.Lo.GetElem(1))
	boundsLo[2] = min(loChild.Lo.GetElem(2), hiChild.Lo.GetElem(2))

	boundsHi := [4]float32{}
	boundsHi[0] = max(loChild.Hi.GetElem(0), hiChild.Hi.GetElem(0))
	boundsHi[1] = max(loChild.Hi.GetElem(1), hiChild.Hi.GetElem(1))
	boundsHi[2] = max(loChild.Hi.GetElem(2), hiChild.Hi.GetElem(2))

	node := t.GetFreeNode()
	node.Type = BVHNodeTypeComposite
	node.Lo = archsimd.LoadFloat32x4(&boundsLo)
	node.Hi = archsimd.LoadFloat32x4(&boundsHi)
	node.LoChild = loChild
	node.HiChild = hiChild

	return node
}

type BVHNodeType int

const (
	BVHNodeTypeConcrete BVHNodeType = iota
	BVHNodeTypeComposite
)

type BVHNode struct {
	Type BVHNodeType
	Lo   archsimd.Float32x4
	Hi   archsimd.Float32x4

	Ref int

	LoChild *BVHNode
	HiChild *BVHNode
}

func (b *BVHNode) Reset(nodeType BVHNodeType, lo, hi fvec3.T, ref int) {
	b.Type = nodeType
	b.Lo = archsimd.LoadFloat32x4((*[4]float32)(&lo))
	b.Hi = archsimd.LoadFloat32x4((*[4]float32)(&hi))
	b.Ref = ref
}

func (b *BVHNode) RayTest(point, inverseSlope archsimd.Float32x4) bool {
	// https://tavianator.com/2011/ray_box.html

	loContacts := b.Lo.Sub(point).Mul(inverseSlope)
	hiContacts := b.Hi.Sub(point).Mul(inverseSlope)

	// Now loContacts[0] is the collision point of the ray and the lo x-plane of
	// the box

	// Now reorder the contacts so we have one register with all the *closest*
	// contacts, and one with all the *furthest* contacts.
	var nearContacts, farContacts [4]float32
	loContacts.Min(hiContacts).Store(&nearContacts)
	loContacts.Max(hiContacts).Store(&farContacts)

	// Now get the furthest near contact, and the nearest far contact.  There is
	// no particularly efficient way to do this with SSE or AVX, so we just do
	// it in normal code.
	furthestNearContact := max(nearContacts[0], max(nearContacts[1], nearContacts[2]))
	nearestFarContact := min(farContacts[0], min(farContacts[1], farContacts[2]))

	return furthestNearContact <= nearestFarContact
}
