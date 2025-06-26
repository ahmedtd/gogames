package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image/color"
	"io/fs"
	"log"
	"os"

	"github.com/ahmedtd/gogames/embeddedassets/departuremonofont"
	"github.com/ahmedtd/gogames/lib/resource/spritesheet"
	"github.com/ahmedtd/gogames/lib/viewport2"
	"github.com/ahmedtd/gogames/lib/vmath/mat22"
	"github.com/ahmedtd/gogames/lib/vmath/mat33"
	"github.com/ahmedtd/gogames/lib/vmath/vec2"
	"github.com/google/subcommands"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

func main() {
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.CommandsCommand(), "")

	subcommands.Register(&ViewCommand{}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}

type ViewCommand struct {
	spriteSheetPath string

	uiFaceSource *text.GoTextFaceSource
	uiNormalFace *text.GoTextFace

	sheet          *spritesheet.T
	showIndex      int
	changeDebounce int
}

var _ subcommands.Command = (*ViewCommand)(nil)
var _ ebiten.Game = (*ViewCommand)(nil)

func (*ViewCommand) Name() string {
	return "view"
}

func (*ViewCommand) Synopsis() string {
	return "Shows a complete spritesheet and associated data"
}

func (*ViewCommand) Usage() string {
	return ``
}

func (c *ViewCommand) SetFlags(f *flag.FlagSet) {
	f.StringVar(&c.spriteSheetPath, "sprite-sheet", "", "Path to a sprite sheet textproto")
}

func (c *ViewCommand) Execute(ctx context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if err := c.do(ctx); err != nil {
		log.Printf("Error: %v", err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

func (c *ViewCommand) do(ctx context.Context) error {
	newSheet, err := spritesheet.LoadFromFS(os.DirFS(".").(fs.ReadFileFS), c.spriteSheetPath)
	if err != nil {
		return fmt.Errorf("while loading spritesheet %q: %v", c.spriteSheetPath, err)
	}
	c.sheet = newSheet

	c.uiFaceSource, err = text.NewGoTextFaceSource(bytes.NewReader(departuremonofont.DepartureMonoOTF))
	if err != nil {
		return fmt.Errorf("while loading Departure Mono font: %w", err)
	}
	c.uiNormalFace = &text.GoTextFace{
		Source: c.uiFaceSource,
		Size:   24,
	}

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("spritetool view")
	if err := ebiten.RunGame(c); err != nil {
		return fmt.Errorf("while running game: %w", err)
	}
	return nil
}

func (c *ViewCommand) Update() error {
	if ebiten.IsKeyPressed(ebiten.KeyR) {
		newSheet, err := spritesheet.LoadFromFS(os.DirFS(".").(fs.ReadFileFS), c.spriteSheetPath)
		if err != nil {
			log.Printf("Could not load spritesheet: %v", err)
		}
		c.sheet = newSheet
	}
	if len(c.sheet.Proto.Sprites) == 0 {
		c.showIndex = 0
	} else if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		if c.changeDebounce > 0 {
			c.changeDebounce--
		} else {
			c.showIndex--
			if c.showIndex < 0 {
				c.showIndex = len(c.sheet.Proto.Sprites) - 1
			}
			c.changeDebounce = 20
		}
	} else if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		if c.changeDebounce > 0 {
			c.changeDebounce--
		} else {
			c.showIndex++
			if c.showIndex == len(c.sheet.Proto.Sprites) {
				c.showIndex = 0
			}
			c.changeDebounce = 20
		}
	} else {
		c.changeDebounce = 0
	}

	return nil
}

func (c *ViewCommand) Draw(screen *ebiten.Image) {
	viewport := viewport2.Viewport{
		Center:             vec2.T{0.0, 0.0},
		Orientation:        mat22.Identity(),
		PixelsPerWorldUnit: 50.0,
	}
	screenWidthPixels, screenHeightPixels := ebiten.WindowSize()
	viewport.ComputeForFrame(screenWidthPixels, screenHeightPixels)
	worldToScreen := viewport.WorldToScreenMatrix

	if c.sheet == nil {
	} else if len(c.sheet.Proto.GetSprites()) == 0 {

	} else {
		entPosition := vec2.T{0, 0}

		modelToWorld := mat33.Transform2(1.0/64.0, mat22.Identity(), entPosition)
		c.sheet.RenderAtWorldPosition(screen, c.showIndex, worldToScreen, modelToWorld)

		entScreenCenter := mat33.MulMV2(worldToScreen, entPosition)
		vector.DrawFilledCircle(screen, float32(entScreenCenter[0]), float32(entScreenCenter[1]), 4, color.RGBA{0, 255, 0, 255}, true)

		const x, y = 20, 20
		infoText := fmt.Sprintf("name=%v", c.sheet.Proto.GetSprites()[c.showIndex].GetName())
		w, h := text.Measure(infoText, c.uiNormalFace, c.uiNormalFace.Size*1.5)
		vector.DrawFilledRect(screen, x, y, float32(w), float32(h), color.RGBA{0x80, 0x80, 0x80, 0xff}, false)
		op := &text.DrawOptions{}
		op.GeoM.Translate(x, y)
		op.LineSpacing = c.uiNormalFace.Size * 1.5
		text.Draw(screen, infoText, c.uiNormalFace, op)
	}
}

func (c *ViewCommand) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}
