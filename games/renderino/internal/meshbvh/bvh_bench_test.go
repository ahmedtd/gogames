package meshbvh

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// randomTriangle makes a randomly placed, randomly sized triangle inside a
// cube of side 2*sceneExtent centered on the origin.
func randomTriangle(rng *rand.Rand, sceneExtent float32) Triangle {
	var center fvec4.T
	for k := range 3 {
		center[k] = (rng.Float32()*2 - 1) * sceneExtent
	}
	size := (0.05 + rng.Float32()*0.45) * sceneExtent / 100

	var vs [3]fvec4.T
	for j := range vs {
		for k := range 3 {
			vs[j][k] = center[k] + (rng.Float32()*2-1)*size
		}
	}
	return NewTriangle(vs[0], vs[1], vs[2])
}

// buildRandomBVH constructs a tree over numTris random triangles from
// randomTriangle.
func buildRandomBVH(rng *rand.Rand, numTris int, sceneExtent float32) *Tree {
	tree := &Tree{}
	for range numTris {
		tree.AddTriangle(randomTriangle(rng, sceneExtent))
	}
	tree.Build()
	return tree
}

// testRay is a ray origin and direction, as passed to Tree.RayTest.
type testRay struct {
	Point fvec4.T
	Slope fvec4.T
}

// randomRay generates a ray with an origin outside the scene, aimed roughly
// at a random point inside it, so that a realistic mix of queries actually
// enter the tree rather than all trivially missing it.
func randomRay(rng *rand.Rand, sceneExtent float32) testRay {
	origin := fvec4.T{
		(rng.Float32()*2 - 1) * sceneExtent * 1.5,
		(rng.Float32()*2 - 1) * sceneExtent * 1.5,
		(rng.Float32()*2 - 1) * sceneExtent * 1.5,
		0,
	}
	target := fvec4.T{
		(rng.Float32()*2 - 1) * sceneExtent,
		(rng.Float32()*2 - 1) * sceneExtent,
		(rng.Float32()*2 - 1) * sceneExtent,
		0,
	}
	slope := fvec4.Normalize(fvec4.SubVV(target, origin))
	return testRay{Point: origin, Slope: slope}
}

// BenchmarkBVHRayTest measures Tree.RayTest query throughput against
// randomly-generated scenes of varying size.  b.N is the number of ray
// queries fired at a fixed scene: the query set is pregenerated before
// b.ResetTimer so RNG cost isn't attributed to RayTest itself, and only
// Tree.RayTest runs inside the timed loop.
//
// Run with, e.g.:
//
//	go test ./games/renderino/internal/meshbvh/ -bench BenchmarkBVHRayTest -benchmem
//	GOEXPERIMENT=simd go test ./games/renderino/internal/meshbvh/ -bench BenchmarkBVHRayTest -benchmem
func BenchmarkBVHRayTest(b *testing.B) {
	for _, numTris := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("tris=%d", numTris), func(b *testing.B) {
			const sceneExtent = 1000.0

			rng := rand.New(rand.NewPCG(1, uint64(numTris)))
			tree := buildRandomBVH(rng, numTris, sceneExtent)

			rays := make([]testRay, b.N)
			for i := range rays {
				rays[i] = randomRay(rng, sceneExtent)
			}

			stack := &Stack{}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tree.RayTest(rays[i].Point, rays[i].Slope, 0, posInf, stack, nil)
			}
		})
	}
}
