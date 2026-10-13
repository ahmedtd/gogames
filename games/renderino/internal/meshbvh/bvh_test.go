package meshbvh

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

var (
	negInf = float32(math.Inf(-1))
	posInf = float32(math.Inf(1))
)

// aimedRay makes a ray from outside the scene aimed at the centroid of a
// random triangle of tree, so that most queries hit something.
func aimedRay(rng *rand.Rand, tree *Tree, sceneExtent float32) testRay {
	tri := &tree.triangles[rng.IntN(len(tree.triangles))]
	target := fvec4.AddVV(tri.V0, fvec4.MulVS(fvec4.AddVV(tri.E1, tri.E2), 1.0/3))
	ray := randomRay(rng, sceneExtent)
	ray.Slope = fvec4.Normalize(fvec4.SubVV(target, ray.Point))
	return ray
}

// bruteForceRayTest tests ray against every triangle of tree, returning the
// nearest hit with tLo < t < tHi.
func bruteForceRayTest(tree *Tree, ray testRay, tLo, tHi float32) (Hit, bool) {
	hit := Hit{Ref: -1, T: tHi}
	for ref := range tree.triangles {
		if t, b1, b2, ok := tree.triangles[ref].RayIntersect(ray.Point, ray.Slope, tLo, hit.T); ok {
			hit = Hit{Ref: ref, T: t, B1: b1, B2: b2}
		}
	}
	return hit, hit.Ref != -1
}

// TestBVHRayTestMatchesBruteForce checks that Tree.RayTest finds the same
// nearest hit as a brute-force scan over every triangle, and that
// Tree.RayOccluded agrees on whether there's any hit, across many
// randomly-generated scenes and rays.  This is the main correctness guard
// for the wide (4-ary) BVH layout: a bug in the struct-of-arrays child
// bounds, the hit-mask handling, the interval culling, or the tree assembly
// would show up here as a missed or farther hit.
func TestBVHRayTestMatchesBruteForce(t *testing.T) {
	const sceneExtent = 1000.0

	for _, numTris := range []int{1, 2, 3, 4, 5, 8, 17, 100, 977, 10_000} {
		rng := rand.New(rand.NewPCG(1, uint64(numTris)))
		tree := buildRandomBVH(rng, numTris, sceneExtent)

		numHits := 0
		for q := 0; q < 200; q++ {
			ray := randomRay(rng, sceneExtent)
			if q%2 == 0 {
				ray = aimedRay(rng, tree, sceneExtent)
			}

			// Sometimes cut the query short of the nearest hit, so that
			// the next one (if any) is found instead.
			tLo, tHi := float32(0), posInf
			if q%3 == 0 {
				if want, ok := bruteForceRayTest(tree, ray, tLo, tHi); ok {
					tLo = want.T
				}
			}

			want, wantOK := bruteForceRayTest(tree, ray, tLo, tHi)
			got, gotOK := tree.RayTest(ray.Point, ray.Slope, tLo, tHi, nil, nil)
			if gotOK != wantOK {
				t.Fatalf("numTris=%d query=%d: RayTest hit = %v, want %v", numTris, q, gotOK, wantOK)
			}
			if wantOK {
				numHits++
				// Ties may be broken differently, so check that the
				// reported triangle really gives the reported hit.
				tt, b1, b2, ok := tree.triangles[got.Ref].RayIntersect(ray.Point, ray.Slope, tLo, tHi)
				if !ok || got.T != want.T || tt != got.T || b1 != got.B1 || b2 != got.B2 {
					t.Fatalf("numTris=%d query=%d: RayTest = %+v, want %+v", numTris, q, got, want)
				}
			}

			if occluded := tree.RayOccluded(ray.Point, ray.Slope, tLo, tHi, nil, nil); occluded != wantOK {
				t.Fatalf("numTris=%d query=%d: RayOccluded = %v, want %v", numTris, q, occluded, wantOK)
			}
		}

		// Guard against a test that passes by never hitting anything.
		if numHits < 50 {
			t.Errorf("numTris=%d: only %d of 200 rays hit", numTris, numHits)
		}
	}
}

// TestBVHRayTestSharedStack checks that queries sharing a Stack report
// exactly what they would with a temporary one, leaving the stack empty
// afterwards.
func TestBVHRayTestSharedStack(t *testing.T) {
	const sceneExtent = 1000.0

	rng := rand.New(rand.NewPCG(3, 0))
	tree := buildRandomBVH(rng, 977, sceneExtent)

	stack := &Stack{}
	for q := 0; q < 200; q++ {
		ray := aimedRay(rng, tree, sceneExtent)

		got, gotOK := tree.RayTest(ray.Point, ray.Slope, 0, posInf, stack, nil)
		want, wantOK := tree.RayTest(ray.Point, ray.Slope, 0, posInf, nil, nil)
		if got != want || gotOK != wantOK {
			t.Fatalf("query=%d: RayTest with shared stack = %+v, want %+v", q, got, want)
		}
		if got, want := tree.RayOccluded(ray.Point, ray.Slope, 0, posInf, stack, nil), tree.RayOccluded(ray.Point, ray.Slope, 0, posInf, nil, nil); got != want {
			t.Fatalf("query=%d: RayOccluded with shared stack = %v, want %v", q, got, want)
		}
		if len(stack.entries) != 0 {
			t.Fatalf("query=%d: stack left with %d entries, want 0", q, len(stack.entries))
		}
	}
}

// TestBVHLeaves checks a tree's leaves: they partition the triangles, all but
// one of them full; each leaf's bounds enclose its triangles' bounds; and
// each leaf's unused lanes hold the degenerate triangle at the origin.
func TestBVHLeaves(t *testing.T) {
	const sceneExtent = 1000.0

	for _, numTris := range []int{1, 2, 3, 4, 5, 7, 8, 9, 17, 100, 977} {
		rng := rand.New(rand.NewPCG(3, uint64(numTris)))
		tree := buildRandomBVH(rng, numTris, sceneExtent)

		// Leaf-size-aligned splits leave only one leaf partly full, so the
		// triangles fit in the fewest leaves possible.
		if want := (numTris + LeafSize - 1) / LeafSize; len(tree.leaves) != want {
			t.Errorf("numTris=%d: got %d leaves, want %d", numTris, len(tree.leaves), want)
		}
		var allRefs []int
		for i := range tree.leaves {
			l := &tree.leaves[i]
			if l.N == 0 || l.N > LeafSize {
				t.Fatalf("numTris=%d: leaf %d has %d triangles", numTris, i, l.N)
			}
			for _, ref := range l.Refs[:l.N] {
				allRefs = append(allRefs, int(ref))
			}
			for lane := int(l.N); lane < LeafSize; lane++ {
				if l.V0X[lane] != 0 || l.V0Y[lane] != 0 || l.V0Z[lane] != 0 ||
					l.E1X[lane] != 0 || l.E1Y[lane] != 0 || l.E1Z[lane] != 0 ||
					l.E2X[lane] != 0 || l.E2Y[lane] != 0 || l.E2Z[lane] != 0 {
					t.Fatalf("numTris=%d: leaf %d's unused lane %d isn't zeroed", numTris, i, lane)
				}
			}
		}
		slices.Sort(allRefs)
		for i, ref := range allRefs {
			if ref != i {
				t.Fatalf("numTris=%d: leaves don't partition the triangles: %v", numTris, allRefs)
			}
		}

		leaves := collectLeaves(tree.Root)
		if len(leaves) != len(tree.leaves) {
			t.Fatalf("numTris=%d: tree has %d leaf nodes, want %d", numTris, len(leaves), len(tree.leaves))
		}
		for i, node := range leaves {
			l := &tree.leaves[i]
			for _, ref := range l.Refs[:l.N] {
				b := tree.triangles[ref].bounds()
				for k := range 3 {
					if b.Lo[k] < node.Lo[k] || b.Hi[k] > node.Hi[k] {
						t.Fatalf("numTris=%d: leaf %d's bounds don't enclose triangle %d", numTris, i, ref)
					}
				}
			}
		}
	}
}

// collectLeaves maps each leaf node's Ref to the node itself.
func collectLeaves(root *Node) map[int]*Node {
	leaves := map[int]*Node{}
	var walk func(node *Node)
	walk = func(node *Node) {
		if node == nil {
			return
		}
		if node.Type == NodeTypeConcrete {
			leaves[node.Ref] = node
			return
		}
		for i := 0; i < node.NumChildren; i++ {
			walk(node.Children[i])
		}
	}
	walk(root)
	return leaves
}
