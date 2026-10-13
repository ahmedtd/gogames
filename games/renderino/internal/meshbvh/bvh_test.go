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

// TestBVHRayTestMatchesBruteForce checks that Tree.RayTest and
// Tree.RayTestUnordered each report exactly the same set of candidate boxes as a brute-force linear scan over every
// leaf actually present in the tree, across many randomly-generated scenes
// and rays.  This is the main correctness guard for the wide (4-ary) BVH
// layout: a bug in the struct-of-arrays child bounds, the hit-mask handling,
// or the tree assembly would show up here as a mismatched hit set long
// before it'd be visible from rendered output.
func TestBVHRayTestMatchesBruteForce(t *testing.T) {
	const sceneExtent = 1000.0

	for _, numBoxes := range []int{1, 2, 3, 4, 5, 8, 17, 100, 977} {
		rng := rand.New(rand.NewPCG(1, uint64(numBoxes)))
		tree := buildRandomBVH(rng, numBoxes, sceneExtent)

		for q := 0; q < 200; q++ {
			ray := randomRay(rng, sceneExtent)

			wantHits := bruteForceRayTest(tree, numBoxes, ray)
			slices.Sort(wantHits)

			for _, method := range []struct {
				name    string
				rayTest func(point, slope fvec4.T, tLo, tHi float32, stack *Stack, stats *Stats, checkRef func(ref int) float32)
			}{
				{"RayTest", tree.RayTest},
				{"RayTestUnordered", tree.RayTestUnordered},
			} {
				var gotHits []int
				method.rayTest(ray.Point, ray.Slope, negInf, posInf, nil, nil, func(ref int) float32 {
					gotHits = append(gotHits, ref)
					return posInf
				})
				slices.Sort(gotHits)

				if !slices.Equal(gotHits, wantHits) {
					t.Fatalf("numBoxes=%d query=%d: Tree.%s = %v, want %v", numBoxes, q, method.name, gotHits, wantHits)
				}
			}
		}
	}
}

// bruteForceRayTest walks every leaf actually present in tree and
// slab-tests it directly against ray, independent of however Tree.RayTest
// internally descends -- the correctness oracle for
// TestBVHRayTestMatchesBruteForce.
func bruteForceRayTest(tree *Tree, numBoxes int, ray testRay) []int {
	inverseSlope := [3]float32{
		1.0 / ray.Slope[0],
		1.0 / ray.Slope[1],
		1.0 / ray.Slope[2],
	}

	var hits []int
	var walk func(node *Node)
	walk = func(node *Node) {
		if node == nil {
			return
		}
		if node.Type == NodeTypeConcrete {
			lo := node.Lo
			hi := node.Hi

			tx1 := (lo[0] - ray.Point[0]) * inverseSlope[0]
			tx2 := (hi[0] - ray.Point[0]) * inverseSlope[0]
			tMin := min(tx1, tx2)
			tMax := max(tx1, tx2)

			ty1 := (lo[1] - ray.Point[1]) * inverseSlope[1]
			ty2 := (hi[1] - ray.Point[1]) * inverseSlope[1]
			tMin = max(tMin, min(ty1, ty2))
			tMax = min(tMax, max(ty1, ty2))

			tz1 := (lo[2] - ray.Point[2]) * inverseSlope[2]
			tz2 := (hi[2] - ray.Point[2]) * inverseSlope[2]
			tMin = max(tMin, min(tz1, tz2))
			tMax = min(tMax, max(tz1, tz2))

			if tMin <= tMax {
				hits = append(hits, node.Ref)
			}
			return
		}
		for i := 0; i < node.NumChildren; i++ {
			walk(node.Children[i])
		}
	}
	walk(tree.Root)

	if len(hits) > numBoxes {
		panic("walked more leaves than boxes in the scene -- tree structure is corrupt")
	}

	return hits
}

// TestBVHRayTestClosestHit checks that a closest-hit query -- where checkRef
// narrows the search interval to the nearest hit found so far -- still finds
// the same nearest box as a brute-force scan.  This guards the interval
// culling and nearest-first ordering in Tree.RayTest: culling too
// aggressively would show up here as a missed or farther hit.  Each box's
// "hit" is the ray parameter at which the ray enters it, clamped to 0 for
// rays starting inside a box.
func TestBVHRayTestClosestHit(t *testing.T) {
	const sceneExtent = 1000.0

	for _, numBoxes := range []int{1, 2, 3, 4, 5, 8, 17, 100, 977, 10_000} {
		rng := rand.New(rand.NewPCG(2, uint64(numBoxes)))
		tree := buildRandomBVH(rng, numBoxes, sceneExtent)
		leaves := collectLeaves(tree.Root)

		for q := 0; q < 200; q++ {
			ray := randomRay(rng, sceneExtent)

			gotT := posInf
			tree.RayTest(ray.Point, ray.Slope, 0, posInf, nil, nil, func(ref int) float32 {
				node := leaves[ref]
				tMin, tMax := slabInterval(node.Lo, node.Hi, ray)
				tMin = max(tMin, 0)
				if tMin <= tMax && tMin < gotT {
					gotT = tMin
				}
				return gotT
			})

			wantT := posInf
			for _, node := range leaves {
				tMin, tMax := slabInterval(node.Lo, node.Hi, ray)
				tMin = max(tMin, 0)
				if tMin <= tMax && tMin < wantT {
					wantT = tMin
				}
			}

			if gotT != wantT {
				t.Fatalf("numBoxes=%d query=%d: closest hit t = %v, want %v", numBoxes, q, gotT, wantT)
			}
		}
	}
}

// collectLeaves maps each leaf's Ref to the leaf itself.
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

// slabInterval returns the range of ray parameters over which ray is inside
// the box [lo, hi]; the ray misses the box if tMin > tMax.
func slabInterval(lo, hi [4]float32, ray testRay) (tMin, tMax float32) {
	tMin, tMax = negInf, posInf
	for k := range 3 {
		inv := 1.0 / ray.Slope[k]
		t1 := (lo[k] - ray.Point[k]) * inv
		t2 := (hi[k] - ray.Point[k]) * inv
		tMin = max(tMin, min(t1, t2))
		tMax = min(tMax, max(t1, t2))
	}
	return tMin, tMax
}

// TestBVHRayTestNestedStack checks that RayTest calls nested inside another's
// checkRef can share its Stack -- the way a scene-level traversal descends
// into each mesh's own tree -- and still report exactly what they would with
// separate stacks, leaving the stack empty afterwards.
func TestBVHRayTestNestedStack(t *testing.T) {
	const sceneExtent = 1000.0

	// Unlike buildRandomBVH's small boxes, these are large enough that most
	// rays hit several boxes in each tree, so traversals actually nest: every
	// outer box spans the whole scene, and inner boxes a tenth of it.
	rng := rand.New(rand.NewPCG(3, 0))
	buildBVH := func(numBoxes int, halfSize float32) *Tree {
		tree := &Tree{}
		for i := range numBoxes {
			var center fvec4.T
			for k := range 3 {
				center[k] = (rng.Float32()*2 - 1) * sceneExtent
			}
			tree.AddElement(AABB{
				Lo: fvec4.T{center[0] - halfSize, center[1] - halfSize, center[2] - halfSize, 0},
				Hi: fvec4.T{center[0] + halfSize, center[1] + halfSize, center[2] + halfSize, 0},
			}, i)
		}
		tree.Build()
		return tree
	}
	outer := buildBVH(17, 2*sceneExtent)
	inner := buildBVH(977, sceneExtent/10)

	// Each outer hit triggers an inner traversal; record the (outer, inner)
	// pairs reported.
	collect := func(ray testRay, stack *Stack) [][2]int {
		var hits [][2]int
		outer.RayTest(ray.Point, ray.Slope, negInf, posInf, stack, nil, func(outerRef int) float32 {
			inner.RayTest(ray.Point, ray.Slope, negInf, posInf, stack, nil, func(innerRef int) float32 {
				hits = append(hits, [2]int{outerRef, innerRef})
				return posInf
			})
			return posInf
		})
		return hits
	}

	stack := &Stack{}
	for q := 0; q < 200; q++ {
		ray := randomRay(rng, sceneExtent)

		got := collect(ray, stack)
		want := collect(ray, nil)
		if !slices.Equal(got, want) {
			t.Fatalf("query=%d: hits with shared stack = %v, want %v", q, got, want)
		}
		if len(stack.entries) != 0 {
			t.Fatalf("query=%d: stack left with %d entries, want 0", q, len(stack.entries))
		}
	}
}
