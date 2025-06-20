package simplespacespritesheet

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"

	"github.com/ahmedtd/gogames/lib/resource/spritesheet"
	"github.com/ahmedtd/gogames/lib/resource/spritesheet/spritesheetpb"
	"github.com/hajimehoshi/ebiten/v2"
	"google.golang.org/protobuf/encoding/prototext"
)

//go:embed simplespace.textproto
var textprotoBytes []byte

//go:embed simpleSpace_sheet@2.png
var texturePNG []byte

const EFFECT_PURPLE int = 0

const EFFECT_YELLOW int = 1

const ENEMY_A int = 2

const ENEMY_B int = 3

const ENEMY_C int = 4

const ENEMY_D int = 5

const ENEMY_E int = 6

const ICON_CROSSLARGE int = 7

const ICON_CROSSSMALL int = 8

const ICON_EXCLAMATIONLARGE int = 9

const ICON_EXCLAMATIONSMALL int = 10

const ICON_PLUSLARGE int = 11

const ICON_PLUSSMALL int = 12

const METEOR_DETAILEDLARGE int = 13

const METEOR_DETAILEDSMALL int = 14

const METEOR_LARGE int = 15

const METEOR_SMALL int = 16

const METEOR_SQUAREDETAILEDLARGE int = 17

const METEOR_SQUAREDETAILEDSMALL int = 18

const METEOR_SQUARELARGE int = 19

const METEOR_SQUARESMALL int = 20

const SATELLITE_A int = 21

const SATELLITE_B int = 22

const SATELLITE_C int = 23

const SATELLITE_D int = 24

const SHIP_A int = 25

const SHIP_B int = 26

const SHIP_C int = 27

const SHIP_D int = 28

const SHIP_E int = 29

const SHIP_F int = 30

const SHIP_G int = 31

const SHIP_H int = 32

const SHIP_I int = 33

const SHIP_J int = 34

const SHIP_K int = 35

const SHIP_L int = 36

const SHIP_SIDESA int = 37

const SHIP_SIDESB int = 38

const SHIP_SIDESC int = 39

const SHIP_SIDESD int = 40

const STAR_LARGE int = 41

const STAR_MEDIUM int = 42

const STAR_SMALL int = 43

const STAR_TINY int = 44

const STATION_A int = 45

const STATION_B int = 46

const STATION_C int = 47

var Sheet *spritesheet.T

func Load() error {
	sheetProto := &spritesheetpb.SpriteSheet{}
	if err := prototext.Unmarshal(textprotoBytes, sheetProto); err != nil {
		return fmt.Errorf("while unmarshaling spritesheet from textproto: %w", err)
	}

	img, err := png.Decode(bytes.NewReader(texturePNG))
	if err != nil {
		return fmt.Errorf("while decoding sprite PNG: %w", err)
	}

	Sheet = spritesheet.New(sheetProto, ebiten.NewImageFromImage(img))

	return nil
}
