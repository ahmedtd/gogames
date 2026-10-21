module github.com/ahmedtd/gogames

go 1.25.0

require (
	github.com/barnex/fmath v0.0.0-20150108074215-ec9671f295c2
	github.com/gogpu/gogpu v0.54.0
	github.com/gogpu/gputypes v0.8.0
	github.com/gogpu/wgpu v0.34.5
	github.com/google/go-cmp v0.6.0
	github.com/google/subcommands v1.2.0
	github.com/hajimehoshi/ebiten/v2 v2.8.5
	go.starlark.net v0.0.0-20230112144946-fae38c8a6d89
	golang.org/x/sync v0.16.0
	google.golang.org/protobuf v1.25.0
)

require (
	github.com/ebitengine/gomobile v0.0.0-20240911145611-4856209ac325 // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/purego v0.8.0 // indirect
	github.com/go-text/typesetting v0.2.0 // indirect
	github.com/go-webgpu/goffi v0.6.3 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gpucontext v0.31.3 // indirect
	github.com/gogpu/naga v0.19.0 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	golang.org/x/image v0.20.0 // indirect
	golang.org/x/mod v0.26.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	golang.org/x/tools v0.35.0 // indirect
)

tool golang.org/x/tools/cmd/stringer

replace github.com/gogpu/wgpu => github.com/ahmedtd/wgpu v0.34.5-rt

replace github.com/gogpu/naga => github.com/ahmedtd/naga v0.20.3-rt

replace github.com/gogpu/gputypes => github.com/ahmedtd/gputypes v0.8.0-rt

replace github.com/gogpu/gogpu => github.com/ahmedtd/gogpu v0.54.0-rt
