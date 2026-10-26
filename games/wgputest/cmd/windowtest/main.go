package main

import (
	"context"
	"embed"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"math"
	"os"
	"unsafe"

	"github.com/ahmedtd/gogames/lib/graphics/pinholecamera"
	"github.com/ahmedtd/gogames/lib/vmath/faffinetransform3"
	"github.com/ahmedtd/gogames/lib/vmath/fmat33"
	"github.com/ahmedtd/gogames/lib/vmath/fvec3"
	"github.com/ahmedtd/gogames/lib/wgpuuniform"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gpu/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"

	_ "github.com/gogpu/wgpu/hal/allbackends"
)

var (
	//go:embed shader.wgsl
	shaderWGSL string
	//go:embed models
	modelsFS embed.FS
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("FAIL: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	app := gogpu.NewApp(
		gogpu.DefaultConfig().
			WithTitle("Hello GoGPU").
			WithSize(800, 600).
			WithContinuousRender(true).
			WithGraphicsAPI(types.GraphicsAPIVulkan).
			WithRequiredFeatures(gputypes.Features(gputypes.FeatureRayQuery)).
			WithRequiredLimits(wgpu.LimitsWithMinimumAccelerationStructureValues(gputypes.DefaultLimits())),
	)

	var ex *Example

	app.OnDraw(func(dc *gogpu.Context) {

		provider := app.DeviceProvider()
		if provider == nil {
			slog.ErrorContext(ctx, "Error: app.DeviceProvider() is nil")
			os.Exit(-1)
		}

		wgpuDevice := provider.Device()
		wgpuQueue := provider.Queue()

		// Check for raytracing support.
		if !app.GPUContextProvider().Features().Contains(gputypes.FeatureRayQuery) {
			slog.ErrorContext(ctx, "Error: selected adapter does not support FeatureRayQuery", slog.Any("adapter", app.GPUContextProvider().AdapterInfo()))
			os.Exit(-1)
		}

		if ex == nil {
			var err error
			ex, err = initExample(
				ctx,
				app.DeviceProvider().SurfaceFormat(),
				wgpuDevice,
				wgpuQueue,
			)
			if err != nil {
				slog.ErrorContext(ctx, "Error creating example", slog.Any("err", err))
				os.Exit(-1)
			}
		}

		ex.animationTime += 0.005

		if err := ex.render(ctx, dc.Width(), dc.Height(), dc.SurfaceView(), wgpuDevice, wgpuQueue); err != nil {
			slog.ErrorContext(ctx, "Error while rendering", slog.Any("err", err))
		}
	})

	if err := app.Run(); err != nil {
		return fmt.Errorf("while running app: %w", err)
	}

	return nil
}

// Uniforms corresponds to the Uniforms struct defined in the wgsl shader.
type Uniforms struct {
	WindowWidth  float32
	WindowHeight float32

	CamPos fvec3.T

	// Orthornormal transform from aperture space to world space.
	//
	// Column 0 is the right vector.  Column 1 is the down vector.  Column 2 is
	// the eye vector.
	CamApertureToWorld fmat33.T
}

func (u Uniforms) bytes() []byte {
	out := []byte{}
	out = wgpuuniform.AppendFloat32(u.WindowWidth, out)
	out = wgpuuniform.AppendFloat32(u.WindowHeight, out)
	out = wgpuuniform.AppendFVec3(&u.CamPos, out)
	out = wgpuuniform.AppendFMat33(&u.CamApertureToWorld, out)

	// The overall alignment of the Uniforms struct is 16, so we need to round
	// the size up to a multiple of 16.
	out = wgpuuniform.PadTo(16, out)

	return out
}

// SceneComponents mirrors the Rust struct of the same name.
type SceneComponents struct {
	vertices                          *wgpu.Buffer
	indices                           *wgpu.Buffer
	geometries                        *wgpu.Buffer
	instances                         *wgpu.Buffer
	bottomLevelAccelerationStructures []*wgpu.Blas
}

// Release frees the scene's GPU resources.
func (s *SceneComponents) Release() {
	for _, b := range s.bottomLevelAccelerationStructures {
		b.Release()
	}
	for _, b := range []*wgpu.Buffer{s.instances, s.geometries, s.indices, s.vertices} {
		if b != nil {
			b.Release()
		}
	}
}

// InstanceEntry mirrors the Rust #[repr(C)] InstanceEntry (16 bytes).
type InstanceEntry struct {
	FirstVertex   uint32
	FirstGeometry uint32
	LastGeometry  uint32
	Pad           uint32
}

// GeometryEntry mirrors the Rust #[repr(C)] GeometryEntry (48 bytes).
type GeometryEntry struct {
	FirstIndex uint32
	_          [3]uint32
	Material   Material
}

// podBytes is the bytemuck::cast_slice equivalent for the example's
// fixed-layout structs (little-endian hosts only, like the GPU upload path).
// u32 converts a scene index or count to uint32, panicking if it doesn't fit.
func u32(n int) uint32 {
	if n < 0 || n > math.MaxUint32 {
		panic(fmt.Sprintf("ray-scene: %d does not fit in uint32", n))
	}
	return uint32(n)
}

func podBytes[T any](v []T) []byte {
	if len(v) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&v[0])), len(v)*int(unsafe.Sizeof(v[0]))) //nolint:gosec // reinterprets a slice of plain-data structs
}

func uploadSceneComponents(device *wgpu.Device, queue *wgpu.Queue, scene *RawSceneComponents) (*SceneComponents, error) {
	geometryBufferContent := make([]GeometryEntry, 0, len(scene.geometries))
	for _, g := range scene.geometries {
		geometryBufferContent = append(geometryBufferContent, GeometryEntry{
			FirstIndex: u32(g.indices.start),
			Material:   g.material,
		})
	}

	instanceBufferContent := make([]InstanceEntry, 0, len(scene.instances))
	for _, inst := range scene.instances {
		instanceBufferContent = append(instanceBufferContent, InstanceEntry{
			FirstVertex:   u32(inst.vertices.start),
			FirstGeometry: u32(inst.geometries.start),
			LastGeometry:  u32(inst.geometries.end),
			Pad:           1, // the Rust example sets _pad: 1
		})
	}

	sc := &SceneComponents{}
	fail := func(what string, err error) (*SceneComponents, error) {
		sc.Release()
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	var err error

	sc.vertices, err = CreateBufferInit(device, "Vertices", podBytes(scene.vertices),
		wgpu.BufferUsageVertex|wgpu.BufferUsageStorage|wgpu.BufferUsageBlasInput)
	if err != nil {
		return fail("create vertex buffer", err)
	}
	sc.indices, err = CreateBufferInit(device, "Indices", U32Bytes(scene.indices),
		wgpu.BufferUsageIndex|wgpu.BufferUsageStorage|wgpu.BufferUsageBlasInput)
	if err != nil {
		return fail("create index buffer", err)
	}
	sc.geometries, err = CreateBufferInit(device, "Geometries", podBytes(geometryBufferContent),
		wgpu.BufferUsageStorage)
	if err != nil {
		return fail("create geometry buffer", err)
	}
	sc.instances, err = CreateBufferInit(device, "Instances", podBytes(instanceBufferContent),
		wgpu.BufferUsageStorage)
	if err != nil {
		return fail("create instance buffer", err)
	}

	indexFormat := gputypes.IndexFormatUint32
	sizeDescriptors := make([][]wgpu.BlasTriangleGeometrySizeDescriptor, 0, len(scene.instances))
	for _, inst := range scene.instances {
		sizeDesc := make([]wgpu.BlasTriangleGeometrySizeDescriptor, 0, inst.geometries.end-inst.geometries.start)
		for i := inst.geometries.start; i < inst.geometries.end; i++ {
			indexCount := u32(scene.geometries[i].indices.end - scene.geometries[i].indices.start)
			sizeDesc = append(sizeDesc, wgpu.BlasTriangleGeometrySizeDescriptor{
				VertexFormat: gputypes.VertexFormatFloat32x3,
				VertexCount:  u32(inst.vertices.end - inst.vertices.start),
				IndexFormat:  &indexFormat,
				IndexCount:   &indexCount,
				Flags:        gputypes.ASGeometryFlagOpaque,
			})
		}

		blas, err := device.CreateBlas(
			&wgpu.CreateBlasDescriptor{
				Flags:      gputypes.ASFlagPreferFastTrace,
				UpdateMode: gputypes.AccelerationStructureUpdateModeBuild,
			},
			wgpu.BlasGeometrySizeDescriptors{Triangles: sizeDesc},
		)
		if err != nil {
			return fail("create BLAS", err)
		}
		sizeDescriptors = append(sizeDescriptors, sizeDesc)
		sc.bottomLevelAccelerationStructures = append(sc.bottomLevelAccelerationStructures, blas)
	}

	buildEntries := make([]wgpu.BlasBuildEntry, 0, len(scene.instances))
	for i, inst := range scene.instances {
		sizeDesc := sizeDescriptors[i]
		triangleGeometries := make([]wgpu.BlasTriangleGeometry, 0, len(sizeDesc))
		for j := range sizeDesc {
			firstIndex := u32(scene.geometries[inst.geometries.start+j].indices.start)
			triangleGeometries = append(triangleGeometries, wgpu.BlasTriangleGeometry{
				Size:         &sizeDesc[j],
				VertexBuffer: sc.vertices,
				FirstVertex:  u32(inst.vertices.start),
				VertexStride: uint64(unsafe.Sizeof(Vertex{})),
				IndexBuffer:  sc.indices,
				FirstIndex:   &firstIndex,
			})
		}
		buildEntries = append(buildEntries, wgpu.BlasBuildEntry{
			Blas:     sc.bottomLevelAccelerationStructures[i],
			Geometry: wgpu.BlasGeometries{Triangles: triangleGeometries},
		})
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{})
	if err != nil {
		return fail("create encoder", err)
	}

	encoder.BuildAccelerationStructures(buildEntries, nil)

	cmd, err := encoder.Finish()
	if err != nil {
		return fail("finish build encoder", err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		return fail("submit build", err)
	}
	return sc, nil
}

func loadScene(device *wgpu.Device, queue *wgpu.Queue) (*SceneComponents, error) {
	models, err := fs.Sub(modelsFS, "models")
	if err != nil {
		return nil, err
	}
	scene, err := loadRawScene(models)
	if err != nil {
		return nil, fmt.Errorf("load models: %w", err)
	}
	return uploadSceneComponents(device, queue, scene)
}

// Example holds the state the Rust Example struct holds.
type Example struct {
	uniforms        Uniforms
	animationTime   float32
	sceneComponents *SceneComponents

	uniformBuf      *wgpu.Buffer
	tlas            *wgpu.Tlas
	pipeline        *wgpu.RenderPipeline
	bindGroup       *wgpu.BindGroup
	shaderModule    *wgpu.ShaderModule
	bindGroupLayout *wgpu.BindGroupLayout
	pipelineLayout  *wgpu.PipelineLayout
}

func (ex *Example) Release() {
	// Ordering is the reverse of the ordering in initExample
	if ex.bindGroup != nil {
		ex.bindGroup.Release()
	}
	if ex.pipeline != nil {
		ex.pipeline.Release()
	}
	if ex.pipelineLayout != nil {
		ex.pipelineLayout.Release()
	}
	if ex.bindGroupLayout != nil {
		ex.bindGroupLayout.Release()
	}
	if ex.shaderModule != nil {
		ex.shaderModule.Release()
	}
	if ex.tlas != nil {
		ex.tlas.Release()
	}
	if ex.uniformBuf != nil {
		ex.uniformBuf.Release()
	}
	if ex.sceneComponents != nil {
		ex.sceneComponents.Release()
	}
}

func initExample(ctx context.Context, format gputypes.TextureFormat, device *wgpu.Device, queue *wgpu.Queue) (returnExample *Example, returnErr error) {
	const sideCount = 8

	ex := &Example{}

	// If we aren't returning the example, we need to release everything that
	// has been partially-constructed.
	defer func() {
		if returnErr != nil {
			ex.Release()
		}
	}()

	sceneComponents, err := loadScene(device, queue)
	if err != nil {
		return nil, fmt.Errorf("while loading scene: %w", err)
	}
	ex.sceneComponents = sceneComponents

	camera := &pinholecamera.PinholeCamera{
		Center:          fvec3.T{0, 0, 0},
		ApertureToWorld: fmat33.Identity(),
		Aperture:        fvec3.T{0.02, 0.018, 0.012},
	}

	// right := fvec3.MulVS(camera.Left(), -1)
	// down := fvec3.MulVS(camera.Up(), -1)
	// eye := camera.Eye()

	// Looking along the world's x-axis, right is -y, down is -z, eye is +x
	//
	// Column 0 is right, Column 1 is down, column 2 is eye.
	apertureToWorld := fmat33.T{
		0, 0, 1,
		-1, 0, 0,
		0, -1, 0,
	}
	// apertureToWorld := fmat33.T{
	// 	0, -1, 0,
	// 	0, 0, -1,
	// 	1, 0, 0,
	// }

	ex.uniforms = Uniforms{
		CamPos:             camera.Center,
		CamApertureToWorld: apertureToWorld,
	}

	if err := ex.uploadUniforms(ctx, device); err != nil {
		return nil, fmt.Errorf("while uploading uniforms: %w", err)
	}

	ex.tlas, err = device.CreateTlas(&wgpu.CreateTlasDescriptor{
		Flags:        gputypes.ASFlagPreferFastTrace,
		UpdateMode:   gputypes.AccelerationStructureUpdateModeBuild,
		MaxInstances: sideCount * sideCount,
	})
	if err != nil {
		return nil, fmt.Errorf("while creating top-level acceleration structure: %w", err)
	}

	ex.shaderModule, err = device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{WGSL: shaderWGSL})
	if err != nil {
		return nil, fmt.Errorf("while creating shader module: %w", err)
	}

	// Rust passes `layout: None` and then calls pipeline.get_bind_group_layout(0).
	// Native Go has no implicit pipeline layouts, so the layout the Rust
	// auto-derivation would produce is spelled out here: every binding is
	// only referenced by fs_main.
	storage := func(binding uint32) gputypes.BindGroupLayoutEntry {
		return gputypes.BindGroupLayoutEntry{
			Binding:    binding,
			Visibility: gputypes.ShaderStageFragment,
			Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeReadOnlyStorage},
		}
	}
	ex.bindGroupLayout, err = device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Buffer:     &gputypes.BufferBindingLayout{Type: gputypes.BufferBindingTypeUniform},
			},
			storage(1),
			storage(2),
			storage(3),
			storage(4),
			{
				Binding:               5,
				Visibility:            gputypes.ShaderStageFragment,
				AccelerationStructure: &gputypes.AccelerationStructureBindingLayout{},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("while creating bind group layout: %w", err)
	}

	ex.pipelineLayout, err = device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{ex.bindGroupLayout},
	})
	if err != nil {
		return nil, fmt.Errorf("while creating pipeline layout: %w", err)
	}

	ex.pipeline, err = device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Layout: ex.pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     ex.shaderModule,
			EntryPoint: "vs_main",
		},
		Fragment: &wgpu.FragmentState{
			Module:     ex.shaderModule,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format:    format,
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
		Primitive:   gputypes.PrimitiveState{Topology: gputypes.PrimitiveTopologyTriangleList},
		Multisample: gputypes.DefaultMultisampleState(),
	})
	if err != nil {
		return nil, fmt.Errorf("while creating render pipeline: %w", err)
	}

	ex.bindGroup, err = device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: ex.bindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: ex.uniformBuf},
			{Binding: 5, AccelerationStructure: ex.tlas},
			{Binding: 1, Buffer: sceneComponents.vertices},
			{Binding: 2, Buffer: sceneComponents.indices},
			{Binding: 3, Buffer: sceneComponents.geometries},
			{Binding: 4, Buffer: sceneComponents.instances},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("while creating bind group: %w", err)
	}

	return ex, nil
}

func (ex *Example) render(ctx context.Context, width int, height int, view *wgpu.TextureView, device *wgpu.Device, queue *wgpu.Queue) error {
	// scene update
	{
		const dist = 3.5
		const sideCount = 2

		// Rust: AnimationTimer::time() returns 0.0 on its first call, which
		// is the only frame the test renders.
		animTime := ex.animationTime
		blases := ex.sceneComponents.bottomLevelAccelerationStructures

		for x := range sideCount {
			for y := range sideCount {
				// blasIndex := (x + y) % len(blases)
				blasIndex := 0

				fx := float32(x)/float32(sideCount-1)*2.0 - 1.0
				fy := float32(y)/float32(sideCount-1)*2.0 - 1.0

				transform := faffinetransform3.Compose(
					faffinetransform3.Translate(fvec3.T{fx * dist, fy * dist, 0}),
					faffinetransform3.AxisAngle(fvec3.T{0, 0, 1}, animTime),
				)

				ex.tlas.SetInstance(x+y*sideCount, wgpu.NewTlasInstance(
					blases[blasIndex],
					transform.As3x4(),
					u32(blasIndex),
					0xff,
				))
			}
		}
	}
	//ex.uniforms.CamPos[2] = 5 * fmath.Sin(ex.animationTime)

	ex.uniforms.WindowWidth = float32(width)
	ex.uniforms.WindowHeight = float32(height)

	if err := ex.uploadUniforms(ctx, device); err != nil {
		return fmt.Errorf("while uploading uniforms: %w", err)
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{})
	if err != nil {
		return fmt.Errorf("create encoder: %w", err)
	}

	encoder.BuildAccelerationStructures(nil, []*wgpu.Tlas{ex.tlas})

	rpass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       view,
			LoadOp:     gputypes.LoadOpClear,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{R: 0, G: 1, B: 0, A: 1},
		}},
	})
	if err != nil {
		return fmt.Errorf("begin render pass: %w", err)
	}
	rpass.SetPipeline(ex.pipeline)
	rpass.SetBindGroup(0, ex.bindGroup, nil)
	// There are no vertex buffers bound, so we are just invoking the vertex
	// stage three times with indices [0, 1, 2].  The vertex shader maps these
	// two a large triangle that covers the viewport.
	rpass.Draw(gputypes.DrawArgs{VertexCount: 3, InstanceCount: 1})
	if err := rpass.End(); err != nil {
		return fmt.Errorf("end render pass: %w", err)
	}

	cmd, err := encoder.Finish()
	if err != nil {
		return fmt.Errorf("finish encoder: %w", err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		return fmt.Errorf("submit: %w", err)
	}
	return nil
}

func (ex *Example) uploadUniforms(ctx context.Context, device *wgpu.Device) (overallErr error) {
	var err error

	uniformBytes := ex.uniforms.bytes()
	if ex.uniformBuf != nil && int(ex.uniformBuf.Size()) != len(uniformBytes) {
		ex.uniformBuf.Release()
		ex.uniformBuf = nil
	}
	if ex.uniformBuf == nil {
		ex.uniformBuf, err = device.CreateBuffer(&wgpu.BufferDescriptor{
			Label:            "Uniforms",
			Size:             uint64(len(uniformBytes)),
			Usage:            wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst | wgpu.BufferUsageMapWrite,
			MappedAtCreation: false,
		})
		if err != nil {
			return fmt.Errorf("while allocating new uniform buffer: %w", err)
		}
	}
	if err := ex.uniformBuf.Map(ctx, wgpu.MapModeWrite, 0, uint64(len(uniformBytes))); err != nil {
		return fmt.Errorf("while mapping uniform buffer: %w", err)
	}
	defer func() {
		if err := ex.uniformBuf.Unmap(); err != nil && !errors.Is(err, wgpu.ErrMapNotMapped) {
			overallErr = fmt.Errorf("while unmapping uniform buffer: %w", err)
		}
	}()

	uniformRange, err := ex.uniformBuf.MappedRange(0, ex.uniformBuf.Size())
	if err != nil {
		return fmt.Errorf("while getting uniform buffer mapped range: %w", err)
	}
	copy(uniformRange.BytesMut(), uniformBytes)

	return nil
}

func alignUp(n, a uint32) uint32 { return (n + a - 1) / a * a }

// CreateBufferInit is the equivalent of wgpu::util::DeviceExt::create_buffer_init:
// it creates a buffer mapped at creation, copies contents in, and unmaps it.
// Like Rust, it pads the size up to a multiple of 4 (COPY_BUFFER_ALIGNMENT).
func CreateBufferInit(device *wgpu.Device, label string, contents []byte, usage gputypes.BufferUsage) (*wgpu.Buffer, error) {
	size := uint64(alignUp(uint32(len(contents)), 4)) //nolint:gosec // example buffers are far below 4 GiB
	buf, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            label,
		Size:             size,
		Usage:            usage,
		MappedAtCreation: true,
	})
	if err != nil {
		return nil, err
	}
	rng, err := buf.MappedRange(0, size)
	if err != nil {
		buf.Release()
		return nil, fmt.Errorf("mapped range: %w", err)
	}
	copy(rng.BytesMut(), contents)
	if err := buf.Unmap(); err != nil {
		buf.Release()
		return nil, fmt.Errorf("unmap: %w", err)
	}
	return buf, nil
}

// U32Bytes returns v as little-endian bytes (bytemuck::cast_slice).
func U32Bytes(v []uint32) []byte {
	b := make([]byte, 0, 4*len(v))
	for _, u := range v {
		b = binary.LittleEndian.AppendUint32(b, u)
	}
	return b
}
