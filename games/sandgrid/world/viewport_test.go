package world

import (
	"fmt"
	"math"
	"shellworld/lib/vmath/vec2"
	"testing"
)

func TestWorldPositionToScreenPosition(t *testing.T) {
	type worldToScreenSubtest struct {
		worldPos      vec2.T
		wantScreenPos vec2.T
	}

	type screenToWorldSubtest struct {
		screenPos    vec2.T
		wantWorldPos vec2.T
	}

	testCases := []struct {
		viewport              Viewport
		worldToScreenSubtests []worldToScreenSubtest
	}{
		{
			viewport: Viewport{
				Center:         vec2.T{0.0, 0.0},
				Angle:          0.0,
				PixelsPerMeter: 1.0,

				ScreenWidthPixels:  1000,
				ScreenHeightPixels: 1000,
			},
			worldToScreenSubtests: []worldToScreenSubtest{
				{
					worldPos:      vec2.T{0, 0},
					wantScreenPos: vec2.T{500, 500},
				},
				{
					worldPos:      vec2.T{500, 0},
					wantScreenPos: vec2.T{1000, 500},
				},
				{
					worldPos:      vec2.T{-500, 0},
					wantScreenPos: vec2.T{0, 500},
				},
				{
					worldPos:      vec2.T{0, 500},
					wantScreenPos: vec2.T{500, 0},
				},
				{
					worldPos:      vec2.T{0, -500},
					wantScreenPos: vec2.T{500, 1000},
				},
			},
		},
		{
			viewport: Viewport{
				Center:         vec2.T{0.0, 0.0},
				Angle:          0.0,
				PixelsPerMeter: 0.5,

				ScreenWidthPixels:  1000,
				ScreenHeightPixels: 1000,
			},
			worldToScreenSubtests: []worldToScreenSubtest{
				{
					worldPos:      vec2.T{0, 0},
					wantScreenPos: vec2.T{500, 500},
				},
				{
					worldPos:      vec2.T{2, 0},
					wantScreenPos: vec2.T{501, 500},
				},
				{
					worldPos:      vec2.T{-2, 0},
					wantScreenPos: vec2.T{499, 500},
				},
				{
					worldPos:      vec2.T{0, 2},
					wantScreenPos: vec2.T{500, 499},
				},
				{
					worldPos:      vec2.T{0, -2},
					wantScreenPos: vec2.T{500, 501},
				},
			},
		},
		{
			viewport: Viewport{
				Center:         vec2.T{0.0, 0.0},
				Angle:          math.Pi / 2.0,
				PixelsPerMeter: 1.0,

				ScreenWidthPixels:  1000,
				ScreenHeightPixels: 1000,
			},
			worldToScreenSubtests: []worldToScreenSubtest{
				{
					worldPos:      vec2.T{0, 0},
					wantScreenPos: vec2.T{500, 500},
				},
				{
					worldPos:      vec2.T{1, 0},
					wantScreenPos: vec2.T{500, 501},
				},
				{
					worldPos:      vec2.T{-1, 0},
					wantScreenPos: vec2.T{500, 499},
				},
				{
					worldPos:      vec2.T{0, 1},
					wantScreenPos: vec2.T{501, 500},
				},
				{
					worldPos:      vec2.T{0, -1},
					wantScreenPos: vec2.T{499, 500},
				},
			},
		},
		{
			viewport: Viewport{
				Center:         vec2.T{-10.0, 5.0},
				Angle:          0,
				PixelsPerMeter: 1.0,

				ScreenWidthPixels:  1000,
				ScreenHeightPixels: 1000,
			},
			worldToScreenSubtests: []worldToScreenSubtest{
				{
					worldPos:      vec2.T{-10, 5},
					wantScreenPos: vec2.T{500, 500},
				},
				{
					worldPos:      vec2.T{-9, 5},
					wantScreenPos: vec2.T{501, 500},
				},
				{
					worldPos:      vec2.T{-11, 5},
					wantScreenPos: vec2.T{499, 500},
				},
				{
					worldPos:      vec2.T{-10, 6},
					wantScreenPos: vec2.T{500, 499},
				},
				{
					worldPos:      vec2.T{-10, 4},
					wantScreenPos: vec2.T{500, 501},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.viewport), func(t *testing.T) {
			for _, stc := range tc.worldToScreenSubtests {
				t.Run(fmt.Sprintf("%v", stc.worldPos), func(t *testing.T) {
					gotScreenPos := tc.viewport.WorldPositionToScreenPosition(stc.worldPos)
					if vec2.Sub(gotScreenPos, stc.wantScreenPos).Norm() >= 0.001 {
						t.Errorf("Bad output for %v; got %v, want %v", stc.worldPos, gotScreenPos, stc.wantScreenPos)
					}

					roundTripWorldPos := tc.viewport.ScreenPositionToWorldPosition(gotScreenPos)
					if vec2.Sub(roundTripWorldPos, stc.worldPos).Norm() >= 0.001 {
						t.Errorf("Screen position %v didn't round-trip; got %v, want %v", gotScreenPos, roundTripWorldPos, stc.worldPos)
					}
				})
			}
		})
	}
}
