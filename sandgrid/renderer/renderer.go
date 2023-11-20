package renderer

import (
	"fmt"
	"log"
	"time"

	"shellworld/americana/resource/imagestamp"
	"shellworld/americana/resource/loader"
	"shellworld/americana/vmath/vec2"
	"shellworld/sandgrid/world"

	"github.com/hajimehoshi/ebiten/v2"
)

type Renderer struct {
	world *world.World

	loader         *loader.ResourceLoader
	sandImageStamp *imagestamp.ImageStamp
}

var _ ebiten.Game = &Renderer{}

func New(world *world.World, loader *loader.ResourceLoader) (*Renderer, error) {
	r := &Renderer{
		world:  world,
		loader: loader,
	}

	var err error
	r.sandImageStamp, err = r.loader.LoadImageStamp("//sand:sand")
	if err != nil {
		return nil, fmt.Errorf("while loading sand image stamp: %w", err)
	}

	return r, nil
}

func (r *Renderer) Update() error {
	rp := r.world.GetRenderPulse()
	if rp == nil {
		log.Printf("no renderpulse objects available")
		return nil
	}

	// rp could contain garbage from its last use, so we need to initialize
	// every field.
	rp.RealNow = time.Now()
	rp.ScreenWidthPixels, rp.ScreenHeightPixels = ebiten.WindowSize()
	rp.MouseButtonLeftDown = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	rp.MouseButtonRightDown = ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	rp.CursorScreenPositionX, rp.CursorScreenPositionY = ebiten.CursorPosition()

	rp.Key1Pressed = ebiten.IsKeyPressed(ebiten.Key1)
	rp.Key2Pressed = ebiten.IsKeyPressed(ebiten.Key2)

	if ok := r.world.Pulse(rp); !ok {
		log.Printf("World inbox is full")
		return nil
	}

	return nil
}

// TODO: Draw likely runs slower than Update.  The world outbox mechanism needs
// to be enhanced so that we don't expect the rate of incoming renderpulses and
// the rate of outgoing renderstates to be the same.
func (r *Renderer) Draw(screen *ebiten.Image) {
	rs := r.world.PollOutbox()
	if rs == nil {
		log.Printf("No RenderState in world outbox")
		return
	}

	worldToScreen := rs.Viewport.WorldToScreenMatrix()

	tileWidthX := 0.1
	tileWidthY := 0.1

	totalWidthX := float64(rs.ResX) * tileWidthX
	totalWidthY := float64(rs.ResY) * tileWidthY

	// Top left of top left tile on the world grid.
	topLeftX := -totalWidthX / 2.0
	topLeftY := totalWidthY / 2.0

	// Center of top left tile on the world grid.
	topLeftCenterX := topLeftX + tileWidthX/2.0
	topLeftCenterY := topLeftY - tileWidthY/2.0

	for cx := 0; cx < rs.ResX; cx++ {
		for cy := 0; cy < rs.ResY; cy++ {
			tileCenter := vec2.T{
				topLeftCenterX + float64(cx)*tileWidthX,
				topLeftCenterY - float64(cy)*tileWidthY,
			}
			tileFacing := vec2.T{1.0, 0.0}

			if rs.SandLayer[cy*rs.ResX+cx] <= 1000 {
				tint := float64(rs.SandLayer[cy*rs.ResX+cx]) / 1000.0
				r.sandImageStamp.RenderOnWorld(screen, worldToScreen, tileCenter, tileFacing, imagestamp.Tint{1.0, 1.0, 1.0, tint})
			} else if rs.SandLayer[cy*rs.ResX+cx] <= 2000 {
				tint := float64(rs.SandLayer[cy*rs.ResX+cx]-1000) / 1000.0
				r.sandImageStamp.RenderOnWorld(screen, worldToScreen, tileCenter, tileFacing, imagestamp.Tint{1.0, 1.0 - tint, 1.0 - tint, 1.0})
			} else {
				tint := 1.0
				r.sandImageStamp.RenderOnWorld(screen, worldToScreen, tileCenter, tileFacing, imagestamp.Tint{1.0, 1.0 - tint, 1.0 - tint, 1.0})
			}
		}
	}

	r.sandImageStamp.RenderOnWorld(screen, worldToScreen, rs.CursorWorldPosition, vec2.T{1, 0}, imagestamp.Tint{1.0, 0.0, 0.0, 1.0})

	r.world.ReleaseRenderState(rs)
}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}
