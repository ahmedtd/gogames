package main

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// buildRandomBVH constructs a BVH tree containing numBoxes randomly placed,
// randomly sized axis-aligned bounding boxes scattered through a cube of side
// 2*sceneExtent centered on the origin.
func buildRandomBVH(rng *rand.Rand, numBoxes int, sceneExtent float32) *BVHTree {
	tree := &BVHTree{}

	nodes := make([]*BVHNode, numBoxes)
	for i := range nodes {
		center := fvec4.T{
			(rng.Float32()*2 - 1) * sceneExtent,
			(rng.Float32()*2 - 1) * sceneExtent,
			(rng.Float32()*2 - 1) * sceneExtent,
			0,
		}
		halfSize := (0.05 + rng.Float32()*0.45) * sceneExtent / 100

		node := tree.GetFreeNode()
		node.Reset(
			BVHNodeTypeConcrete,
			fvec4.T{center[0] - halfSize, center[1] - halfSize, center[2] - halfSize, 0},
			fvec4.T{center[0] + halfSize, center[1] + halfSize, center[2] + halfSize, 0},
			i,
		)
		nodes[i] = node
	}

	tree.BuildFromElements(nodes)
	return tree
}

// randomRay generates a ray with an origin outside the scene, aimed roughly
// at a random point inside it, so that a realistic mix of queries actually
// intersect boxes rather than all trivially missing the whole tree.
func randomRay(rng *rand.Rand, sceneExtent float32) Ray {
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
	return Ray{Point: origin, Slope: slope}
}

// BenchmarkBVHRayTest measures BVHTree.RayTest query throughput against
// randomly-generated scenes of varying size.  b.N is the number of ray
// queries fired at a fixed scene: the query set is pregenerated before
// b.ResetTimer so RNG cost isn't attributed to RayTest itself, and only
// BVHTree.RayTest runs inside the timed loop.
//
// Run with, e.g.:
//
//	go test ./games/renderino/cmd/renderino/ -bench BenchmarkBVHRayTest -benchmem
//	GOEXPERIMENT=simd go test ./games/renderino/cmd/renderino/ -bench BenchmarkBVHRayTest -benchmem
//
// to compare the portable and SIMD BVH implementations, or add
// -benchtime 500000x to pin the exact number of queries per sub-benchmark.
func BenchmarkBVHRayTest(b *testing.B) {
	for _, numBoxes := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("boxes=%d", numBoxes), func(b *testing.B) {
			const sceneExtent = 1000.0

			rng := rand.New(rand.NewPCG(1, uint64(numBoxes)))
			tree := buildRandomBVH(rng, numBoxes, sceneExtent)

			rays := make([]Ray, b.N)
			for i := range rays {
				rays[i] = randomRay(rng, sceneExtent)
			}

			hits := 0

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tree.RayTest(rays[i], func(ref int) {
					hits++
				})
			}
			b.StopTimer()

			//b.ReportMetric(float64(hits)/float64(b.N), "hits/query")
		})
	}
}
