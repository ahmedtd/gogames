//go:build goexperiment.simd && amd64

package main

import (
	"math/bits"
	"simd/archsimd"
	"slices"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// bvhBranchingFactor is the number of children a composite BVH node can
// have.  Testing all of a node's children against a ray is one SIMD pass
// over a [bvhBranchingFactor]float32 lane per bound, so this is tied to the
// SIMD register width used by bvhRayTest4 (Float32x4 -- 4 lanes).
const bvhBranchingFactor = 4

type BVHTree struct {
	Root *BVHNode

	FreeList                  []*BVHNode
	AllocationsSinceLastClear int
}

// Clear moves all nodes from the tree to the free list.
func (t *BVHTree) Clear() {
	workStack := []*BVHNode{t.Root}
	for len(workStack) != 0 {
		cur := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		if cur == nil {
			continue
		}

		for i := 0; i < cur.NumChildren; i++ {
			workStack = append(workStack, cur.Children[i])
		}
		*cur = BVHNode{}

		t.FreeList = append(t.FreeList, cur)
	}

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
	if t.Root == nil {
		return
	}

	// Every node pushed onto workStack below has already been bounds-tested
	// as one lane of its parent's bvhRayTest4 call. The root has no parent,
	// so if it's a leaf, it must be tested directly here instead.
	if t.Root.Type == BVHNodeTypeConcrete {
		if bvhLeafRayTest(t.Root.Lo, t.Root.Hi, ray.Point, ray.Slope) {
			checkRef(t.Root.Ref)
		}
		return
	}

	pointX := archsimd.BroadcastFloat32x4(ray.Point[0])
	pointY := archsimd.BroadcastFloat32x4(ray.Point[1])
	pointZ := archsimd.BroadcastFloat32x4(ray.Point[2])

	invSlopeX := archsimd.BroadcastFloat32x4(1.0 / ray.Slope[0])
	invSlopeY := archsimd.BroadcastFloat32x4(1.0 / ray.Slope[1])
	invSlopeZ := archsimd.BroadcastFloat32x4(1.0 / ray.Slope[2])

	workStack := make([]*BVHNode, 0, 32)
	workStack = append(workStack, t.Root)
	for len(workStack) != 0 {
		node := workStack[len(workStack)-1]
		workStack = workStack[:len(workStack)-1]

		switch node.Type {
		case BVHNodeTypeConcrete:
			// This node was already bounds-tested as one lane of its
			// parent's bvhRayTest4 call, so it can be reported directly.
			checkRef(node.Ref)
		case BVHNodeTypeComposite:
			hitMask := bvhRayTest4(
				&node.ChildLoX, &node.ChildLoY, &node.ChildLoZ,
				&node.ChildHiX, &node.ChildHiY, &node.ChildHiZ,
				pointX, pointY, pointZ,
				invSlopeX, invSlopeY, invSlopeZ,
			)
			// Padding lanes beyond NumChildren hold garbage; mask them off.
			hitMask &= uint8(1<<node.NumChildren) - 1
			for hitMask != 0 {
				i := bits.TrailingZeros8(hitMask)
				hitMask &= hitMask - 1
				workStack = append(workStack, node.Children[i])
			}
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
	slices.SortFunc(elements, func(a, b *BVHNode) int {
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
	children := make([]*BVHNode, 0, bvhBranchingFactor)
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
// struct-of-arrays bounds used by bvhRayTest4 and the node's own overall
// bounds (the union of its children's), the latter so that this node's
// bounds are available if it's in turn absorbed as someone else's child.
func (t *BVHTree) makeComposite(children []*BVHNode) *BVHNode {
	node := t.GetFreeNode()
	node.Type = BVHNodeTypeComposite
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

type BVHNodeType int

const (
	BVHNodeTypeConcrete BVHNodeType = iota
	BVHNodeTypeComposite
)

type BVHNode struct {
	Type BVHNodeType

	// Valid when Type == BVHNodeTypeConcrete: this node's own bounds and
	// scene-element reference.  Also valid (as the union of children's
	// bounds) when Type == BVHNodeTypeComposite, since this node might in
	// turn be some other node's child.
	Lo  [4]float32
	Hi  [4]float32
	Ref int

	// Valid when Type == BVHNodeTypeComposite.  Children holds NumChildren
	// child pointers (trailing slots unused).  ChildLoX/Y/Z and
	// ChildHiX/Y/Z store the same children's bounds in struct-of-arrays
	// form -- one lane per child -- so bvhRayTest4 can test all of a node's
	// children with a handful of SIMD ops instead of one test per child.
	NumChildren                  int
	Children                     [bvhBranchingFactor]*BVHNode
	ChildLoX, ChildLoY, ChildLoZ [bvhBranchingFactor]float32
	ChildHiX, ChildHiY, ChildHiZ [bvhBranchingFactor]float32
}

func (b *BVHNode) Reset(nodeType BVHNodeType, lo, hi fvec4.T, ref int) {
	b.Type = nodeType
	b.Lo = ([4]float32)(lo)
	b.Hi = ([4]float32)(hi)
	b.Ref = ref
}

// bvhRayTest4 tests a ray (given as broadcast point/inverse-slope lanes) against
// up to bvhBranchingFactor sibling AABBs at once, one per lane of the
// childLo*/childHi* struct-of-arrays bounds, using the slab method
// (https://tavianator.com/2011/ray_box.html).  It returns a bitmask with bit i
// set if lane i was hit; lanes beyond the node's real child count hold
// whatever was last stored there and must be masked off by the caller.
func bvhRayTest4(
	childLoX, childLoY, childLoZ *[bvhBranchingFactor]float32,
	childHiX, childHiY, childHiZ *[bvhBranchingFactor]float32,
	pointX, pointY, pointZ archsimd.Float32x4,
	invSlopeX, invSlopeY, invSlopeZ archsimd.Float32x4,
) uint8 {
	loX := archsimd.LoadFloat32x4(childLoX)
	loY := archsimd.LoadFloat32x4(childLoY)
	loZ := archsimd.LoadFloat32x4(childLoZ)
	hiX := archsimd.LoadFloat32x4(childHiX)
	hiY := archsimd.LoadFloat32x4(childHiY)
	hiZ := archsimd.LoadFloat32x4(childHiZ)

	txLo := loX.Sub(pointX).Mul(invSlopeX)
	txHi := hiX.Sub(pointX).Mul(invSlopeX)
	tyLo := loY.Sub(pointY).Mul(invSlopeY)
	tyHi := hiY.Sub(pointY).Mul(invSlopeY)
	tzLo := loZ.Sub(pointZ).Mul(invSlopeZ)
	tzHi := hiZ.Sub(pointZ).Mul(invSlopeZ)

	tMin := txLo.Min(txHi)
	tMax := txLo.Max(txHi)
	tMin = tMin.Max(tyLo.Min(tyHi))
	tMax = tMax.Min(tyLo.Max(tyHi))
	tMin = tMin.Max(tzLo.Min(tzHi))
	tMax = tMax.Min(tzLo.Max(tzHi))

	return tMin.LessEqual(tMax).ToBits()
}

// bvhLeafRayTest tests a ray against a single AABB directly, using the slab
// method (https://tavianator.com/2011/ray_box.html).  It only exists to
// handle a scene with exactly one element, where the tree's root is that
// leaf itself and so was never tested as part of a parent's bvhRayTest4
// call; this is not a hot path, so it's plain scalar code rather than SIMD.
func bvhLeafRayTest(lo, hi fvec4.T, point, slope fvec4.T) bool {
	invSlope := fvec4.T{1.0 / slope[0], 1.0 / slope[1], 1.0 / slope[2], 0}

	tx1 := (lo[0] - point[0]) * invSlope[0]
	tx2 := (hi[0] - point[0]) * invSlope[0]
	tMin := min(tx1, tx2)
	tMax := max(tx1, tx2)

	ty1 := (lo[1] - point[1]) * invSlope[1]
	ty2 := (hi[1] - point[1]) * invSlope[1]
	tMin = max(tMin, min(ty1, ty2))
	tMax = min(tMax, max(ty1, ty2))

	tz1 := (lo[2] - point[2]) * invSlope[2]
	tz2 := (hi[2] - point[2]) * invSlope[2]
	tMin = max(tMin, min(tz1, tz2))
	tMax = min(tMax, max(tz1, tz2))

	return tMin <= tMax
}
