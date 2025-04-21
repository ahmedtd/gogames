package simplespacespritesheet

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"
	"shellworld/lib/resource/spritesheet"

	"github.com/hajimehoshi/ebiten/v2"
)

var EFFECT_PURPLE = &spritesheet.AtlasEntry{
	X:      688,
	Y:      160,
	Width:  64,
	Height: 127,
}

var EFFECT_YELLOW = &spritesheet.AtlasEntry{
	X:      608,
	Y:      0,
	Width:  64,
	Height: 127,
}

var ENEMY_A = &spritesheet.AtlasEntry{
	X:      400,
	Y:      256,
	Width:  96,
	Height: 96,
}

var ENEMY_B = &spritesheet.AtlasEntry{
	X:      496,
	Y:      192,
	Width:  96,
	Height: 96,
}

var ENEMY_C = &spritesheet.AtlasEntry{
	X:      0,
	Y:      0,
	Width:  128,
	Height: 64,
}

var ENEMY_D = &spritesheet.AtlasEntry{
	X:      512,
	Y:      0,
	Width:  96,
	Height: 96,
}

var ENEMY_E = &spritesheet.AtlasEntry{
	X:      416,
	Y:      96,
	Width:  96,
	Height: 96,
}

var ICON_CROSSLARGE = &spritesheet.AtlasEntry{
	X:      496,
	Y:      288,
	Width:  96,
	Height: 96,
}

var ICON_CROSSSMALL = &spritesheet.AtlasEntry{
	X:      676,
	Y:      384,
	Width:  64,
	Height: 64,
}

var ICON_EXCLAMATIONLARGE = &spritesheet.AtlasEntry{
	X:      512,
	Y:      96,
	Width:  96,
	Height: 96,
}

var ICON_EXCLAMATIONSMALL = &spritesheet.AtlasEntry{
	X:      608,
	Y:      127,
	Width:  64,
	Height: 64,
}

var ICON_PLUSLARGE = &spritesheet.AtlasEntry{
	X:      496,
	Y:      384,
	Width:  96,
	Height: 96,
}

var ICON_PLUSSMALL = &spritesheet.AtlasEntry{
	X:      672,
	Y:      96,
	Width:  64,
	Height: 64,
}

var METEOR_DETAILEDLARGE = &spritesheet.AtlasEntry{
	X:      592,
	Y:      288,
	Width:  96,
	Height: 96,
}

var METEOR_DETAILEDSMALL = &spritesheet.AtlasEntry{
	X:      688,
	Y:      287,
	Width:  64,
	Height: 64,
}

var METEOR_LARGE = &spritesheet.AtlasEntry{
	X:      416,
	Y:      0,
	Width:  96,
	Height: 96,
}

var METEOR_SMALL = &spritesheet.AtlasEntry{
	X:      400,
	Y:      448,
	Width:  64,
	Height: 64,
}

var METEOR_SQUAREDETAILEDLARGE = &spritesheet.AtlasEntry{
	X:      592,
	Y:      192,
	Width:  96,
	Height: 96,
}

var METEOR_SQUAREDETAILEDSMALL = &spritesheet.AtlasEntry{
	X:      736,
	Y:      96,
	Width:  64,
	Height: 64,
}

var METEOR_SQUARELARGE = &spritesheet.AtlasEntry{
	X:      304,
	Y:      192,
	Width:  96,
	Height: 96,
}

var METEOR_SQUARESMALL = &spritesheet.AtlasEntry{
	X:      216,
	Y:      192,
	Width:  64,
	Height: 64,
}

var SATELLITE_A = &spritesheet.AtlasEntry{
	X:      104,
	Y:      296,
	Width:  104,
	Height: 88,
}

var SATELLITE_B = &spritesheet.AtlasEntry{
	X:      0,
	Y:      368,
	Width:  104,
	Height: 104,
}

var SATELLITE_C = &spritesheet.AtlasEntry{
	X:      112,
	Y:      184,
	Width:  104,
	Height: 72,
}

var SATELLITE_D = &spritesheet.AtlasEntry{
	X:      0,
	Y:      296,
	Width:  104,
	Height: 72,
}

var SHIP_A = &spritesheet.AtlasEntry{
	X:      676,
	Y:      448,
	Width:  64,
	Height: 48,
}

var SHIP_B = &spritesheet.AtlasEntry{
	X:      464,
	Y:      448,
	Width:  32,
	Height: 64,
}

var SHIP_C = &spritesheet.AtlasEntry{
	X:      400,
	Y:      192,
	Width:  96,
	Height: 64,
}

var SHIP_D = &spritesheet.AtlasEntry{
	X:      208,
	Y:      448,
	Width:  96,
	Height: 64,
}

var SHIP_E = &spritesheet.AtlasEntry{
	X:      320,
	Y:      96,
	Width:  96,
	Height: 96,
}

var SHIP_F = &spritesheet.AtlasEntry{
	X:      320,
	Y:      0,
	Width:  96,
	Height: 96,
}

var SHIP_G = &spritesheet.AtlasEntry{
	X:      304,
	Y:      384,
	Width:  96,
	Height: 96,
}

var SHIP_H = &spritesheet.AtlasEntry{
	X:      304,
	Y:      288,
	Width:  96,
	Height: 96,
}

var SHIP_I = &spritesheet.AtlasEntry{
	X:      672,
	Y:      0,
	Width:  64,
	Height: 96,
}

var SHIP_J = &spritesheet.AtlasEntry{
	X:      224,
	Y:      96,
	Width:  96,
	Height: 96,
}

var SHIP_K = &spritesheet.AtlasEntry{
	X:      736,
	Y:      0,
	Width:  64,
	Height: 96,
}

var SHIP_L = &spritesheet.AtlasEntry{
	X:      208,
	Y:      352,
	Width:  96,
	Height: 96,
}

var SHIP_SIDESA = &spritesheet.AtlasEntry{
	X:      592,
	Y:      384,
	Width:  84,
	Height: 106,
}

var SHIP_SIDESB = &spritesheet.AtlasEntry{
	X:      120,
	Y:      64,
	Width:  104,
	Height: 104,
}

var SHIP_SIDESC = &spritesheet.AtlasEntry{
	X:      104,
	Y:      384,
	Width:  104,
	Height: 104,
}

var SHIP_SIDESD = &spritesheet.AtlasEntry{
	X:      128,
	Y:      0,
	Width:  96,
	Height: 64,
}

var STAR_LARGE = &spritesheet.AtlasEntry{
	X:      208,
	Y:      256,
	Width:  96,
	Height: 96,
}

var STAR_MEDIUM = &spritesheet.AtlasEntry{
	X:      224,
	Y:      0,
	Width:  96,
	Height: 96,
}

var STAR_SMALL = &spritesheet.AtlasEntry{
	X:      528,
	Y:      480,
	Width:  32,
	Height: 32,
}

var STAR_TINY = &spritesheet.AtlasEntry{
	X:      496,
	Y:      480,
	Width:  32,
	Height: 32,
}

var STATION_A = &spritesheet.AtlasEntry{
	X:      400,
	Y:      352,
	Width:  96,
	Height: 96,
}

var STATION_B = &spritesheet.AtlasEntry{
	X:      0,
	Y:      184,
	Width:  112,
	Height: 112,
}

var STATION_C = &spritesheet.AtlasEntry{
	X:      0,
	Y:      64,
	Width:  120,
	Height: 120,
}

//go:embed simpleSpace_sheet@2.png
var texturePNG []byte

var Sheet *spritesheet.T

func Load() error {
	img, err := png.Decode(bytes.NewReader(texturePNG))
	if err != nil {
		return fmt.Errorf("while decoding sprite PNG: %w", err)
	}

	Sheet = &spritesheet.T{
		Image: ebiten.NewImageFromImage(img),
	}

	return nil
}
