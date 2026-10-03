package main

import (
	"testing"

	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

func TestAABBRayContactPoint(t *testing.T) {
	box := AABB{Lo: fvec4.T{-1, -1, -1}, Hi: fvec4.T{1, 1, 1}}

	cases := []struct {
		name     string
		ray      Ray
		wantMode ContactMode
		wantT    float32
		wantP    fvec4.T
	}{
		{
			name:     "enter through x-lo",
			ray:      Ray{Point: fvec4.T{-5, 0.5, 0.25}, Slope: fvec4.T{2, 0, 0}},
			wantMode: ContactModeEnter,
			wantT:    2,
			wantP:    fvec4.T{-1, 0.5, 0.25},
		},
		{
			name:     "exit through z-hi from inside",
			ray:      Ray{Point: fvec4.T{0.5, -0.5, 0}, Slope: fvec4.T{0, 0, 1}},
			wantMode: ContactModeExit,
			wantT:    1,
			wantP:    fvec4.T{0.5, -0.5, 1},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := box.RayContact(tc.ray, 0, 1000)
			if c.Mode != tc.wantMode {
				t.Fatalf("Mode = %v, want %v", c.Mode, tc.wantMode)
			}
			if !approxEqual(c.T, tc.wantT) {
				t.Errorf("T = %v, want %v", c.T, tc.wantT)
			}
			for k := range 4 {
				if !approxEqual(c.P[k], tc.wantP[k]) {
					t.Errorf("P = %v, want %v", c.P, tc.wantP)
					break
				}
			}
		})
	}
}
