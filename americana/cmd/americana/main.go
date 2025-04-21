package main

// Dependency on ebitengine pulls in glfw, etc.
//
// For now, run with:
//
//    go run ./americana/cmd/americana

import (
	"flag"
	"fmt"
	"log"
	"time"

	"shellworld/americana/embeddedassets/medievaltilesheet"
	"shellworld/americana/world/world"
	"shellworld/lib/vmath/vec2"
	"shellworld/resource/imagestamp"
	"shellworld/resource/loader"
	"shellworld/resource/tilesheet"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	assetDir = flag.String("asset-dir", "./americana/rawassets", "Asset directory to use.")
)

func main() {
	flag.Parse()

	log.Printf("--asset-dir=%q", *assetDir)

	if err := do(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func do() error {
	resourceLoader, err := loader.New(*assetDir)
	if err != nil {
		return fmt.Errorf("while creating resource loader: %w", err)
	}

	theWorld := &world.World{
		Viewport: &world.Viewport{
			Center:         vec2.T{0.0, 0.0},
			Angle:          0.0,
			PixelsPerMeter: 50.0,
		},
	}

	renderer, err := NewRenderer(resourceLoader, theWorld)
	if err != nil {
		return fmt.Errorf("while creating renderer: %w", err)
	}
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Game")
	if err := ebiten.RunGame(renderer); err != nil {
		return fmt.Errorf("while running game: %w", err)
	}

	return nil
}

type Renderer struct {
	resourceLoader *loader.ResourceLoader
	world          *world.World

	latestRenderState *world.RenderState

	playerImageStamp  *imagestamp.ImageStamp
	bootImageStamp    *imagestamp.ImageStamp
	gridDotImageStamp *imagestamp.ImageStamp

	medievalTileSheet *tilesheet.T
}

var _ ebiten.Game = &Renderer{}

func NewRenderer(resourceLoader *loader.ResourceLoader, w *world.World) (*Renderer, error) {
	r := &Renderer{
		resourceLoader:    resourceLoader,
		world:             w,
		latestRenderState: &world.RenderState{},
	}

	var err error
	r.playerImageStamp, err = r.resourceLoader.LoadImageStamp("//playermarker:playermarker")
	if err != nil {
		return nil, fmt.Errorf("while loading player image stamp: %w", err)
	}

	r.bootImageStamp, err = r.resourceLoader.LoadImageStamp("//boot:boot")
	if err != nil {
		return nil, fmt.Errorf("while loading boot image stamp: %w", err)
	}

	r.gridDotImageStamp, err = r.resourceLoader.LoadImageStamp("//gridmarker:gridmarker")
	if err != nil {
		return nil, fmt.Errorf("while loading grid dot image stamp: %w", err)
	}

	r.medievalTileSheet, err = medievaltilesheet.Load()
	if err != nil {
		return nil, fmt.Errorf("while loading medieval tile sheet: %w", err)
	}

	return r, nil
}

func (r *Renderer) Update() error {
	now := time.Now()

	uiInput := &world.UIInput{
		UpdatedAt:   now,
		KeyAPressed: ebiten.IsKeyPressed(ebiten.KeyA),
		KeyDPressed: ebiten.IsKeyPressed(ebiten.KeyD),
		KeySPressed: ebiten.IsKeyPressed(ebiten.KeyS),
		KeyWPressed: ebiten.IsKeyPressed(ebiten.KeyW),
	}

	uiInput.ScreenWidthPixels, uiInput.ScreenHeightPixels = ebiten.WindowSize()

	// Eventually the world update will run in a different goroutine, so we need
	// to be careful about how we move information between the renderer and the
	// world.
	r.world.Update(uiInput, r.latestRenderState)

	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	worldToScreen := r.latestRenderState.Viewport.WorldToScreenMatrix()

	// Render a grid
	for x := -10; x <= 10; x++ {
		for y := -10; y <= 10; y++ {
			r.medievalTileSheet.RenderAtWorldPosition(screen, int(medievaltilesheet.TileDirt1), worldToScreen, vec2.T{float64(x), float64(y)}, vec2.T{1.0, 0.0})
		}
	}

	// Render a leafy tree at 1,0 and a conifer at 0,1
	// r.medievalTileSheet.RenderAtWorldPosition(screen, int(medievaltilesheet.GrayRockSmall), worldToScreen, vec2.T{1.0, 0.0}, vec2.T{1.0, 0.0})
	// r.medievalTileSheet.RenderAtWorldPosition(screen, int(medievaltilesheet.BrownRockSmall), worldToScreen, vec2.T{0.0, 1.0}, vec2.T{1.0, 0.0})

	for _, v := range r.latestRenderState.AntRenderStates {
		if !v.Live {
			continue
		}
		r.medievalTileSheet.RenderAtWorldPosition(screen, int(medievaltilesheet.WomanBlue), worldToScreen, v.Location, v.Facing)
	}

	for _, v := range r.latestRenderState.FoodRenderStates {
		if !v.Live {
			continue
		}
		r.medievalTileSheet.RenderAtWorldPosition(screen, int(medievaltilesheet.BrownRockSmall), worldToScreen, v.Location, v.Facing)
	}

	for _, v := range r.latestRenderState.ScentRenderStates {
		r.playerImageStamp.RenderOnWorld(screen, worldToScreen, v.Location, vec2.T{1.0, 0.0}, imagestamp.Tint{1.0, 1.0, 1.0, 1.0})
	}
}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}
