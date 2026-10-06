package bvh

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// TestBVHLeafGroups checks a tree built with LeafSize > 1: its leaf groups
// partition the elements, all but one of them full; each group leaf's bounds
// enclose its elements' bounds; and RayTest and RayTestUnordered report a
// group for every element whose bounds the ray overlaps.
func TestBVHLeafGroups(t *testing.T) {
	const sceneExtent = 1000.0
	const leafSize = 4

	for _, numBoxes := range []int{1, 2, 3, 4, 5, 7, 8, 9, 17, 100, 977} {
		rng := rand.New(rand.NewPCG(3, uint64(numBoxes)))

		tree := &Tree{LeafSize: leafSize}
		elements := make([]*Node, numBoxes)
		var los, his [][4]float32
		for i := range elements {
			center := fvec4.T{
				(rng.Float32()*2 - 1) * sceneExtent,
				(rng.Float32()*2 - 1) * sceneExtent,
				(rng.Float32()*2 - 1) * sceneExtent,
				0,
			}
			halfSize := (0.05 + rng.Float32()*0.45) * sceneExtent / 100
			lo := fvec4.T{center[0] - halfSize, center[1] - halfSize, center[2] - halfSize, 0}
			hi := fvec4.T{center[0] + halfSize, center[1] + halfSize, center[2] + halfSize, 0}

			elements[i] = &Node{}
			elements[i].Reset(NodeTypeConcrete, lo, hi, i)
			los = append(los, lo)
			his = append(his, hi)
		}
		tree.BuildFromElements(elements)

		// Leaf-size-aligned splits leave only one group partly full, so
		// the elements fit in the fewest groups possible.
		if want := (numBoxes + leafSize - 1) / leafSize; len(tree.LeafGroups) != want {
			t.Errorf("numBoxes=%d: got %d leaf groups, want %d", numBoxes, len(tree.LeafGroups), want)
		}
		var allRefs []int
		for g, group := range tree.LeafGroups {
			if len(group) == 0 || len(group) > leafSize {
				t.Fatalf("numBoxes=%d: group %d has %d elements", numBoxes, g, len(group))
			}
			allRefs = append(allRefs, group...)
		}
		slices.Sort(allRefs)
		for i, ref := range allRefs {
			if ref != i {
				t.Fatalf("numBoxes=%d: leaf groups don't partition the elements: %v", numBoxes, allRefs)
			}
		}

		leaves := collectLeaves(tree.Root)
		if len(leaves) != len(tree.LeafGroups) {
			t.Fatalf("numBoxes=%d: tree has %d leaves, want %d", numBoxes, len(leaves), len(tree.LeafGroups))
		}
		for g, leaf := range leaves {
			for _, ref := range tree.LeafGroups[g] {
				for k := range 3 {
					if los[ref][k] < leaf.Lo[k] || his[ref][k] > leaf.Hi[k] {
						t.Fatalf("numBoxes=%d: group %d's bounds don't enclose element %d", numBoxes, g, ref)
					}
				}
			}
		}

		for q := 0; q < 200; q++ {
			ray := randomRay(rng, sceneExtent)

			var wantHits []int
			for i := range numBoxes {
				if tMin, tMax := slabInterval(los[i], his[i], ray); tMin <= tMax {
					wantHits = append(wantHits, i)
				}
			}

			for _, method := range []struct {
				name    string
				rayTest func(point, slope fvec4.T, tLo, tHi float32, stack *Stack, stats *Stats, checkRef func(ref int) float32)
			}{
				{"RayTest", tree.RayTest},
				{"RayTestUnordered", tree.RayTestUnordered},
			} {
				reported := map[int]bool{}
				method.rayTest(ray.Point, ray.Slope, negInf, posInf, nil, nil, func(g int) float32 {
					for _, ref := range tree.LeafGroups[g] {
						reported[ref] = true
					}
					return posInf
				})
				for _, ref := range wantHits {
					if !reported[ref] {
						t.Fatalf("numBoxes=%d query=%d: Tree.%s didn't report the group of hit element %d", numBoxes, q, method.name, ref)
					}
				}
			}
		}
	}
}
