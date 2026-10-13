package meshbvh

import (
	"math/rand/v2"
	"testing"

	"github.com/ahmedtd/gogames/embeddedassets/kenneycastlekit"
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// TestLeafTrisMatchesTriangle checks that leafTris.rayIntersect gives, lane
// for lane, exactly the result of Triangle.RayIntersect on that lane's
// triangle, for rays aimed at every leaf of a tree over the ballista: some at
// random points near the leaf, and some at its triangles' vertices and edge
// midpoints, where the inside/outside tests are on a knife edge.
func TestLeafTrisMatchesTriangle(t *testing.T) {
	mesh, err := kenneycastlekit.LoadSiegeBallista(true)
	if err != nil {
		t.Fatalf("while loading ballista: %v", err)
	}
	tree := &Tree{}
	for _, tri := range mesh.Tris {
		var vs [3]fvec4.T
		for j := range 3 {
			v := mesh.Vertices[tri.VertexIndices[j]]
			vs[j] = fvec4.T{v[0], v[1], v[2], 0}
		}
		tree.AddTriangle(NewTriangle(vs[0], vs[1], vs[2]))
	}
	tree.Build()

	rng := rand.New(rand.NewPCG(1, 3))
	numHits := 0
	for g := range tree.leaves {
		p := &tree.leaves[g]

		var targets []fvec4.T
		for lane := range int(p.N) {
			tri := &tree.triangles[p.Refs[lane]]
			v1 := fvec4.AddVV(tri.V0, tri.E1)
			v2 := fvec4.AddVV(tri.V0, tri.E2)
			targets = append(targets,
				tri.V0, v1, v2,
				fvec4.MulVS(fvec4.AddVV(tri.V0, v1), 0.5),
				fvec4.MulVS(fvec4.AddVV(v1, v2), 0.5),
				fvec4.MulVS(fvec4.AddVV(v2, tri.V0), 0.5),
			)
			for range 4 {
				b1, b2 := rng.Float32()*1.2-0.1, rng.Float32()*1.2-0.1
				targets = append(targets, fvec4.AddVV(tri.V0, fvec4.AddVV(fvec4.MulVS(tri.E1, b1), fvec4.MulVS(tri.E2, b2))))
			}
		}

		for _, target := range targets {
			origin := fvec4.AddVV(target, fvec4.UniformSphereDistribution(rng, 5))
			origin[3] = 0
			slope := fvec4.SubVV(target, origin)
			// Some rays end short of the target, and some start beyond it.
			queryLo, queryHi := float32(0), float32(1000)
			switch rng.IntN(4) {
			case 0:
				queryHi = 0.999
			case 1:
				queryLo = 1.001
			}

			var gotT, gotB1, gotB2 [LeafSize]float32
			mask := p.rayIntersect(origin, slope, queryLo, queryHi, &gotT, &gotB1, &gotB2)
			if mask>>p.N != 0 {
				t.Fatalf("leaf %d: hit mask %04b reports unused lanes (N=%d)", g, mask, p.N)
			}
			for lane := range int(p.N) {
				wantT, wantB1, wantB2, wantOK := tree.triangles[p.Refs[lane]].RayIntersect(origin, slope, queryLo, queryHi)
				gotOK := mask&(1<<lane) != 0
				if gotOK != wantOK {
					t.Fatalf("leaf %d lane %d: hit = %v, want %v", g, lane, gotOK, wantOK)
				}
				if !wantOK {
					continue
				}
				numHits++
				if gotT[lane] != wantT || gotB1[lane] != wantB1 || gotB2[lane] != wantB2 {
					t.Fatalf("leaf %d lane %d: (t, b1, b2) = (%v, %v, %v), want (%v, %v, %v)",
						g, lane, gotT[lane], gotB1[lane], gotB2[lane], wantT, wantB1, wantB2)
				}
			}
		}
	}

	// Guard against a test that passes by never hitting anything.
	if numHits < 1000 {
		t.Errorf("only %d lanes hit; the test isn't exercising hits", numHits)
	}
	t.Logf("%d lane hits", numHits)
}
