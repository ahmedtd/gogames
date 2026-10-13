package bvh

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// randomPlanes makes between 1 and 6 planes through random points in the
// scene, with random orientations.
func randomPlanes(rng *rand.Rand, sceneExtent float32) []fvec4.T {
	planes := make([]fvec4.T, 1+rng.IntN(6))
	for i := range planes {
		var n, p fvec4.T
		for k := range 3 {
			n[k] = rng.Float32()*2 - 1
			p[k] = (rng.Float32()*2 - 1) * sceneExtent
		}
		n = fvec4.Normalize(n)
		planes[i] = fvec4.T{n[0], n[1], n[2], -fvec4.IProd(n, p)}
	}
	return planes
}

// bruteForceFrustumTest tests every leaf of tree directly against planes.
func bruteForceFrustumTest(tree *Tree, planes []fvec4.T) []int {
	var hits []int
leaves:
	for ref, leaf := range collectLeaves(tree.Root) {
		for _, p := range planes {
			// Test every corner of the leaf's bounds.
			inside := false
			for c := range 8 {
				corner := leaf.Lo
				for k := range 3 {
					if c&(1<<k) != 0 {
						corner[k] = leaf.Hi[k]
					}
				}
				if p[0]*corner[0]+p[1]*corner[1]+p[2]*corner[2]+p[3] >= 0 {
					inside = true
				}
			}
			if !inside {
				continue leaves
			}
		}
		hits = append(hits, ref)
	}
	return hits
}

// TestBVHFrustumTestMatchesBruteForce checks that Tree.FrustumTest reports
// exactly the leaves that aren't entirely outside one of the planes.
func TestBVHFrustumTestMatchesBruteForce(t *testing.T) {
	const sceneExtent = 1000.0

	for _, numBoxes := range []int{1, 2, 3, 4, 5, 8, 17, 100, 977} {
		rng := rand.New(rand.NewPCG(7, uint64(numBoxes)))
		tree := buildRandomBVH(rng, numBoxes, sceneExtent)

		for q := 0; q < 200; q++ {
			planes := randomPlanes(rng, sceneExtent)

			var gotHits []int
			tree.FrustumTest(planes, nil, nil, func(ref int) {
				gotHits = append(gotHits, ref)
			})
			slices.Sort(gotHits)

			wantHits := bruteForceFrustumTest(tree, planes)
			slices.Sort(wantHits)

			if !slices.Equal(gotHits, wantHits) {
				t.Fatalf("numBoxes=%d query=%d: Tree.FrustumTest = %v, want %v", numBoxes, q, gotHits, wantHits)
			}
		}
	}
}

// TestBVHFrustumTestNestedStack checks that FrustumTest calls nested inside
// another's checkRef can share its Stack, and leave it empty afterwards.
func TestBVHFrustumTestNestedStack(t *testing.T) {
	const sceneExtent = 1000.0

	rng := rand.New(rand.NewPCG(8, 0))
	outer := buildRandomBVH(rng, 17, sceneExtent)
	inner := buildRandomBVH(rng, 977, sceneExtent)

	collect := func(planes []fvec4.T, stack *Stack) [][2]int {
		var hits [][2]int
		outer.FrustumTest(planes, stack, nil, func(outerRef int) {
			inner.FrustumTest(planes, stack, nil, func(innerRef int) {
				hits = append(hits, [2]int{outerRef, innerRef})
			})
		})
		return hits
	}

	stack := &Stack{}
	for q := 0; q < 200; q++ {
		planes := randomPlanes(rng, sceneExtent)

		got := collect(planes, stack)
		want := collect(planes, nil)
		if !slices.Equal(got, want) {
			t.Fatalf("query=%d: hits with shared stack = %v, want %v", q, got, want)
		}
		if len(stack.entries) != 0 {
			t.Fatalf("query=%d: stack left with %d entries, want 0", q, len(stack.entries))
		}
	}
}

// TestBVHFrustumTestNearestFirst checks that leaves strung out along the
// first plane's normal are reported nearest-first.  (Ordering is only between
// siblings, so it's only global when sibling subtrees don't overlap along the
// normal, as here.)
func TestBVHFrustumTestNearestFirst(t *testing.T) {
	tree := &Tree{}
	const numBoxes = 100
	rng := rand.New(rand.NewPCG(9, 0))
	for _, x := range rng.Perm(numBoxes) {
		lo := fvec4.T{float32(x), 0, 0, 0}
		hi := fvec4.T{float32(x) + 0.5, lo[1] + 1, lo[2] + 1, 0}
		tree.AddElement(AABB{Lo: lo, Hi: hi}, x)
	}
	tree.Build()

	var got []int
	tree.FrustumTest([]fvec4.T{{1, 0, 0, 0}}, nil, nil, func(ref int) {
		got = append(got, ref)
	})
	if !slices.IsSorted(got) || len(got) != numBoxes {
		t.Errorf("FrustumTest reported %v; want all %d leaves in increasing order", got, numBoxes)
	}
}
