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

const (
	EFFECT_PURPLE              int = 0
	EFFECT_YELLOW              int = 1
	ENEMY_A                    int = 2
	ENEMY_B                    int = 3
	ENEMY_C                    int = 4
	ENEMY_D                    int = 5
	ENEMY_E                    int = 6
	ICON_CROSSLARGE            int = 7
	ICON_CROSSSMALL            int = 8
	ICON_EXCLAMATIONLARGE      int = 9
	ICON_EXCLAMATIONSMALL      int = 10
	ICON_PLUSLARGE             int = 11
	ICON_PLUSSMALL             int = 12
	METEOR_DETAILEDLARGE       int = 13
	METEOR_DETAILEDSMALL       int = 14
	METEOR_LARGE               int = 15
	METEOR_SMALL               int = 16
	METEOR_SQUAREDETAILEDLARGE int = 17
	METEOR_SQUAREDETAILEDSMALL int = 18
	METEOR_SQUARELARGE         int = 19
	METEOR_SQUARESMALL         int = 20
	SATELLITE_A                int = 21
	SATELLITE_B                int = 22
	SATELLITE_C                int = 23
	SATELLITE_D                int = 24
	SHIP_A                     int = 25
	SHIP_B                     int = 26
	SHIP_C                     int = 27
	SHIP_D                     int = 28
	SHIP_E                     int = 29
	SHIP_F                     int = 30
	SHIP_G                     int = 31
	SHIP_H                     int = 32
	SHIP_I                     int = 33
	SHIP_J                     int = 34
	SHIP_K                     int = 35
	SHIP_L                     int = 36
	SHIP_SIDESA                int = 37
	SHIP_SIDESB                int = 38
	SHIP_SIDESC                int = 39
	SHIP_SIDESD                int = 40
	STAR_LARGE                 int = 41
	STAR_MEDIUM                int = 42
	STAR_SMALL                 int = 43
	STAR_TINY                  int = 44
	STATION_A                  int = 45
	STATION_B                  int = 46
	STATION_C                  int = 47
)

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
