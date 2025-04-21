package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"shellworld/games/sandgrid/renderer"
	"shellworld/games/sandgrid/world"
	"shellworld/lib/resource/loader"

	"github.com/hajimehoshi/ebiten/v2"
)

// Dependency on ebitengine pulls in glfw, etc.
//
// For now, run with:
//
//     go run ./sandgrid/cmd/sandgrid

var (
	assetDir = flag.String("asset-dir", "./games/sandgrid/rawassets", "Asset directory to use.")
)

func main() {
	flag.Parse()

	log.Printf("--asset-dir=%q", *assetDir)

	if err := do(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func do() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	loader, err := loader.New(*assetDir)
	if err != nil {
		return fmt.Errorf("while creating resource loader: %w", err)
	}

	world := world.New(128, 128)

	renderer, err := renderer.New(world, loader)
	if err != nil {
		return fmt.Errorf("while creating renderer: %w", err)
	}

	go world.Run(ctx)

	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("SandGrid")
	if err := ebiten.RunGame(renderer); err != nil {
		return fmt.Errorf("while running game: %w", err)
	}

	return nil
}
