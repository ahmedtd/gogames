package main

// Dependency on ebitengine pulls in glfw, etc.
//
// For now, run with:
//
//    go run ./games/renderino/cmd/renderino

import (
	"flag"
	"fmt"
	"log"
	"time"

	"shellworld/americana/world/world"
	"shellworld/vmath/vec3"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	flag.Parse()

	if err := do(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func do() error {
	renderer, err := NewRenderer()
	if err != nil {
		return fmt.Errorf("while creating renderer: %w", err)
	}
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("renderino")
	if err := ebiten.RunGame(renderer); err != nil {
		return fmt.Errorf("while running game: %w", err)
	}

	return nil
}

type Renderer struct {
	LatestRenderState *RenderState

	Buf []byte
}

var _ ebiten.Game = &Renderer{}

func NewRenderer() (*Renderer, error) {
	r := &Renderer{}
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

	return nil
}

func (r *Renderer) Draw(screen *ebiten.Image) {
	wx, wy := ebiten.WindowSize()
	if len(r.Buf) != wx*wy {
		r.Buf = make([]byte, wx*wy*4)
	}

	for px := 0; px < screen.Bounds().Dx(); px++ {
		for py := 0; py < screen.Bounds().Dy(); py++ {
			color := r.LatestRenderState.RenderPixel(px, py, wx, wy)
			r.Buf[py*wx*4+px*4+0] = byte(color >> (0 * 8))
			r.Buf[py*wx*4+px*4+0] = byte(color >> (1 * 8))
			r.Buf[py*wx*4+px*4+0] = byte(color >> (2 * 8))
			r.Buf[py*wx*4+px*4+0] = byte(color >> (3 * 8))
		}
	}

	screen.WritePixels(r.Buf)
}

func (r *Renderer) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

type RenderState struct {
	Camera *Camera

	Spheres     []Sphere
	PointLights []PointLight
}

func (r *RenderState) RenderPixel(px, py, wx, wy int) int {

}

type Camera struct {
	Center  vec3.T
	Forward vec3.T
	Left    vec3.T
}

type Sphere struct {
	Center vec3.T
}

type PointLight struct {
	Center vec3.T
}
