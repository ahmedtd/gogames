package main

import (
	"math/rand/v2"
	"testing"

	"github.com/ahmedtd/gogames/embeddedassets/kenneycastlekit"
	"github.com/ahmedtd/gogames/lib/graphics/renderable3"
	"github.com/ahmedtd/gogames/lib/vmath/fmat44"
	"github.com/ahmedtd/gogames/lib/vmath/fvec2"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"

	"github.com/barnex/fmath"
)

func approxEqual(a, b float32) bool {
	return fmath.Abs(a-b) < 1e-4
}

// TestMeshInstanceTransformedTriangle fires a ray at a single textured
// triangle placed with a scale and translation, and checks the contact
// parameter, point, normal, and texture lookup.
func TestMeshInstanceTransformedTriangle(t *testing.T) {
	// A 2x1 texture: left texel red, right texel green.
	tex := &renderable3.UncompressedImage{
		Width:   2,
		Height:  1,
		Samples: []float32{1, 0, 0, 1, 0, 1, 0, 1},
	}
	mesh := &renderable3.T{
		Vertices:      []fvec3.T{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}},
		TextureCoords: []fvec2.T{{0, 0}, {1, 0}, {0, 1}},
		Normals:       []fvec3.T{{0, 0, 1}},
		Tris: []renderable3.Tri{{
			VertexIndices:  [3]int{0, 1, 2},
			TextureIndices: [3]int{0, 1, 2},
			NormalIndices:  [3]int{0, 0, 0},
		}},
		Diffuse: tex,
	}
	model := NewMeshModel(mesh)

	// Scale by 2, then move up to z = 5.
	inst := NewMeshInstance(model, fmat44.MulMM(translateMat44(fvec4.T{0, 0, 5}), scaleMat44(2)))

	if inst.Lo != (fvec4.T{0, 0, 5}) || inst.Hi != (fvec4.T{2, 2, 5}) {
		t.Errorf("instance bounds = %v, %v; want [0 0 5], [2 2 5]", inst.Lo, inst.Hi)
	}

	cases := []struct {
		name      string
		ray       Ray
		wantMode  ContactMode
		wantT     float32
		wantP     fvec4.T
		wantN     fvec4.T
		wantColor RGBPower
	}{
		{
			name:      "from above, left texel",
			ray:       Ray{Point: fvec4.T{0.5, 0.5, 10}, Slope: fvec4.T{0, 0, -1}},
			wantMode:  ContactModeEnter,
			wantT:     5,
			wantP:     fvec4.T{0.5, 0.5, 5},
			wantN:     fvec4.T{0, 0, 1},
			wantColor: RGBPower{1, 0, 0},
		},
		{
			name:      "from below, right texel",
			ray:       Ray{Point: fvec4.T{1.5, 0.25, 0}, Slope: fvec4.T{0, 0, 2}},
			wantMode:  ContactModeExit,
			wantT:     2.5,
			wantP:     fvec4.T{1.5, 0.25, 5},
			wantN:     fvec4.T{0, 0, -1},
			wantColor: RGBPower{0, 1, 0},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := inst.RayContact(tc.ray, 0, 1000, nil)
			if c.Mode != tc.wantMode {
				t.Fatalf("Mode = %v, want %v", c.Mode, tc.wantMode)
			}
			if !approxEqual(c.T, tc.wantT) {
				t.Errorf("T = %v, want %v", c.T, tc.wantT)
			}
			for k := range 4 {
				if !approxEqual(c.P[k], tc.wantP[k]) || !approxEqual(c.N[k], tc.wantN[k]) {
					t.Errorf("P, N = %v, %v; want %v, %v", c.P, c.N, tc.wantP, tc.wantN)
					break
				}
			}
			if c.Albedo != tc.wantColor {
				t.Errorf("Albedo = %v, want %v", c.Albedo, tc.wantColor)
			}
		})
	}

	// Outside the triangle's hypotenuse.
	if c := inst.RayContact(Ray{Point: fvec4.T{1.5, 1.5, 10}, Slope: fvec4.T{0, 0, -1}}, 0, 1000, nil); c.Mode != ContactModeMiss {
		t.Errorf("ray outside triangle: Mode = %v, want miss", c.Mode)
	}
}

// TestMeshInstanceMatchesBruteForce checks that the BVH-accelerated
// MeshInstance.RayContact finds the same nearest hit as testing every
// triangle of the ballista directly.
func TestMeshInstanceMatchesBruteForce(t *testing.T) {
	mesh, err := kenneycastlekit.LoadSiegeBallista(true)
	if err != nil {
		t.Fatalf("while loading ballista: %v", err)
	}
	model := NewMeshModel(mesh)

	modelToWorld := fmat44.MulMM(
		translateMat44(fvec4.T{3, -2, 1}),
		fmat44.MulMM(axisAngleMat44(fvec4.Normalize(fvec4.T{1, 2, 3}), 0.7), scaleMat44(4)),
	)
	inst := NewMeshInstance(model, modelToWorld)

	rng := rand.New(rand.NewPCG(1, 2))
	numHits := 0
	for q := 0; q < 2000; q++ {
		// Aim from a random point on a sphere around the instance towards a
		// random point inside its bounds.
		center := fvec4.MulVS(fvec4.AddVV(inst.Lo, inst.Hi), 0.5)
		origin := fvec4.AddVV(center, fvec4.UniformSphereDistribution(rng, 20))
		origin[3] = 0
		target := fvec4.T{}
		for k := range 3 {
			target[k] = inst.Lo[k] + rng.Float32()*(inst.Hi[k]-inst.Lo[k])
		}
		ray := Ray{Point: origin, Slope: fvec4.SubVV(target, origin)}

		got := inst.RayContact(ray, 0, 1000, nil)

		// Brute force in model space.
		w2m := fmat44.Inverse(modelToWorld)
		mp := fmat44.MulMV(w2m, fvec4.T{origin[0], origin[1], origin[2], 1})
		mp[3] = 0
		ms := fmat44.MulMV(w2m, ray.Slope)
		wantHit := false
		wantT := float32(1000)
		for i := range model.Tris {
			if tt, _, _, ok := model.Tris[i].RayIntersect(mp, ms, 0, wantT); ok {
				wantHit, wantT = true, tt
			}
		}

		if (got.Mode != ContactModeMiss) != wantHit {
			t.Fatalf("query %d: got hit=%v, want hit=%v", q, got.Mode != ContactModeMiss, wantHit)
		}
		if wantHit {
			numHits++
			if got.T != wantT {
				t.Fatalf("query %d: got T=%v, want %v", q, got.T, wantT)
			}
		}
	}

	// Make sure the test is actually exercising hits.
	if numHits < 100 {
		t.Errorf("only %d of 2000 rays hit the ballista", numHits)
	}
}
