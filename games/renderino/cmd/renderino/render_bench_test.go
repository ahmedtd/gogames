package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ahmedtd/gogames/lib/vmath/fmat44"
	"github.com/ahmedtd/gogames/lib/vmath/fvec4"
)

// benchPose is a fixed camera placement for BenchmarkRenderFrame.
type benchPose struct {
	name   string
	center fvec4.T // Camera position.
	target fvec4.T // Point the camera looks at.
}

var benchPoses = []benchPose{
	// Up against the middle ballista, so that most rays hit its mesh.
	{name: "closeup", center: fvec4.T{-3.2, 0, 2}, target: fvec4.T{0, 0, 1.2}},
	// All three ballistas, with boids scattered across the ground plane.
	{name: "overview", center: fvec4.T{-16, 0, 9}, target: fvec4.T{0, 0, 1}},
}

// The benchmark resolution matches the app's default window size.
const benchRX, benchRY = 1024, 768

var poseImageDir = flag.String("pose-image-dir", "", "If set, TestBenchPoses writes a PNG of each benchmark pose to this directory.")

// newBenchRenderer builds a headless renderer showing the scene at its
// initial simulation state, viewed from pose.  Rendering uses GOMAXPROCS
// threads, so `go test -cpu` controls the thread count.
func newBenchRenderer(tb testing.TB, pose benchPose) *Renderer {
	tb.Helper()

	r, err := newHeadlessRenderer(runtime.GOMAXPROCS(0))
	if err != nil {
		tb.Fatalf("while creating renderer: %v", err)
	}
	r.updateRenderState(r.PreviousWorldState)

	cam := r.LatestRenderState.Camera
	cam.Center = pose.center
	cam.ApertureToWorld = fmat44.Identity()
	cam.SetEye(fvec4.SubVV(pose.target, pose.center))

	return r
}

// BenchmarkRenderFrame measures full-frame raytracing throughput (one op is
// one frame, including exposure and RGBA conversion) from fixed camera poses,
// for before/after comparisons with benchstat:
//
//	GOEXPERIMENT=simd go test ./games/renderino/cmd/renderino -run XXX -bench RenderFrame -count 10 > old.txt
//	... make changes ...
//	GOEXPERIMENT=simd go test ./games/renderino/cmd/renderino -run XXX -bench RenderFrame -count 10 > new.txt
//	benchstat old.txt new.txt
//
// Each pose is rendered with primary rays both raytraced and replaced by
// rasterization, and with and without shadows.
//
// It also reports per-ray work counts (BVH nodes visited, inner nodes visited
// -- one SIMD box test each -- triangle tests, and scene BVH leaves visited,
// each of which is one query of a mesh instance's own tree) and the fraction
// of rays that hit something, which change with the algorithm but not with
// machine speed or noise.
func BenchmarkRenderFrame(b *testing.B) {
	for _, pose := range benchPoses {
		for _, primary := range []string{"raytrace", "raster"} {
			for _, shadows := range []bool{true, false} {
				b.Run(fmt.Sprintf("pose=%s/primary=%s/shadows=%v", pose.name, primary, shadows), func(b *testing.B) {
					r := newBenchRenderer(b, pose)
					r.LatestRenderState.RasterPrimary = primary == "raster"
					r.LatestRenderState.Shadows = shadows

					// The first frame allocates tiles and buffers.
					r.renderFrame(benchRX, benchRY)

					var stats RayStats
					for b.Loop() {
						r.renderFrame(benchRX, benchRY)
						stats.Add(&r.FrameStats)
					}

					b.ReportMetric(float64(b.N)*benchRX*benchRY/b.Elapsed().Seconds()/1e6, "Mrays/s")

					rays := float64(stats.Rays)
					nodesVisited := stats.SceneBVH.NodesVisited() + stats.MeshBVH.NodesVisited()
					innerVisited := stats.SceneBVH.InnerVisited + stats.MeshBVH.InnerVisited
					b.ReportMetric(float64(nodesVisited)/rays, "nodes/ray")
					b.ReportMetric(float64(innerVisited)/rays, "inner/ray")
					b.ReportMetric(float64(stats.MeshBVH.TriTests)/rays, "tris/ray")
					b.ReportMetric(float64(stats.SceneBVH.LeafRefs)/rays, "sleaves/ray")
					b.ReportMetric(float64(stats.RayHits)/rays, "hitfrac")
					b.ReportMetric(float64(stats.ShadowRays)/rays, "shadows/ray")
					b.ReportMetric(float64(stats.Raster.Tris)/rays, "rtris/ray")
					b.ReportMetric(float64(stats.Raster.TrisSetUp)/rays, "rsetup/ray")
					b.ReportMetric(float64(stats.Raster.SamplesTested)/rays, "rtested/ray")
					b.ReportMetric(float64(stats.Raster.SamplesHit)/rays, "rwritten/ray")
				})
			}
		}
	}
}

// TestBenchPoses renders one frame from each benchmark pose.  With
// -pose-image-dir, it writes them out as PNGs, to check what each pose
// actually benchmarks.
func TestBenchPoses(t *testing.T) {
	for _, pose := range benchPoses {
		t.Run(pose.name, func(t *testing.T) {
			r := newBenchRenderer(t, pose)
			r.renderFrame(benchRX, benchRY)

			if *poseImageDir == "" {
				return
			}

			img := &image.RGBA{
				Pix:    r.buf,
				Stride: benchRX * 4,
				Rect:   image.Rect(0, 0, benchRX, benchRY),
			}
			f, err := os.Create(filepath.Join(*poseImageDir, pose.name+".png"))
			if err != nil {
				t.Fatalf("while creating pose image: %v", err)
			}
			defer f.Close()
			if err := png.Encode(f, img); err != nil {
				t.Fatalf("while encoding pose image: %v", err)
			}
		})
	}
}
