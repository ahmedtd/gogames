package main

import (
	"testing"

	"github.com/barnex/fmath"
)

// renderPowers renders one frame from pose at rx by ry, returning each
// pixel's light power (before exposure), row-major.
func renderPowers(t *testing.T, pose benchPose, rx, ry int, rasterPrimary, cullBackFaces bool) []RGBPower {
	t.Helper()
	r := newBenchRenderer(t, pose)
	r.LatestRenderState.RasterPrimary = rasterPrimary
	r.LatestRenderState.CullBackFaces = cullBackFaces
	r.renderFrame(rx, ry)

	powers := make([]RGBPower, rx*ry)
	for _, tile := range r.tiles {
		tileSizeX := tile.LimX - tile.SrcX
		for py := tile.SrcY; py < tile.LimY; py++ {
			for px := tile.SrcX; px < tile.LimX; px++ {
				powers[py*rx+px] = tile.RGBPowerBuf[(py-tile.SrcY)*tileSizeX+(px-tile.SrcX)]
			}
		}
	}
	return powers
}

// TestRasterPrimaryMatchesRaytrace renders each benchmark pose with primary
// rays raytraced and rasterized, and checks that they differ only in a tiny
// fraction of pixels (along silhouettes, where rasterization and raytracing
// can disagree by a hair about which triangle a ray hits first).
func TestRasterPrimaryMatchesRaytrace(t *testing.T) {
	const rx, ry = 320, 240
	for _, pose := range benchPoses {
		for _, cull := range []bool{false, true} {
			want := renderPowers(t, pose, rx, ry, false, false)
			got := renderPowers(t, pose, rx, ry, true, cull)

			var differ int
			for i := range want {
				for c := range 3 {
					if fmath.Abs(got[i][c]-want[i][c]) > 1e-3*max(1, want[i][c]) {
						differ++
						break
					}
				}
			}
			t.Logf("pose=%s cullBackFaces=%v: %d of %d pixels differ", pose.name, cull, differ, rx*ry)
			if float64(differ) > 2e-3*rx*ry {
				t.Errorf("pose=%s cullBackFaces=%v: %d of %d pixels differ; want at most 0.2%%", pose.name, cull, differ, rx*ry)
			}
		}
	}
}
