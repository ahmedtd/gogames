package fantasyui

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

//go:embed fantasyui.textproto
var textprotoBytes []byte

//go:embed fantasyui.png
var texturePNG []byte

const (
	BANNER_CLASSIC_CURTAINIMG             int = 0
	BANNER_HANGINGIMG                     int = 1
	BANNER_MODERNIMG                      int = 2
	BUTTON_BROWN_CLOSEIMG                 int = 3
	BUTTON_BROWNIMG                       int = 4
	BUTTON_GREY_CLOSEIMG                  int = 5
	BUTTON_GREYIMG                        int = 6
	BUTTON_RED_CLOSEIMG                   int = 7
	BUTTON_REDIMG                         int = 8
	CHECKBOX_BEIGE_CHECKEDIMG             int = 9
	CHECKBOX_BEIGE_CROSSIMG               int = 10
	CHECKBOX_BEIGE_EMPTYIMG               int = 11
	CHECKBOX_BROWN_CHECKEDIMG             int = 12
	CHECKBOX_BROWN_CROSSIMG               int = 13
	CHECKBOX_BROWN_EMPTYIMG               int = 14
	CHECKBOX_GREY_CHECKEDIMG              int = 15
	CHECKBOX_GREY_CROSSIMG                int = 16
	CHECKBOX_GREY_EMPTYIMG                int = 17
	HEXAGON_BROWN_DARKIMG                 int = 18
	HEXAGON_BROWNIMG                      int = 19
	HEXAGON_DAMAGED_BROWN_DARKIMG         int = 20
	HEXAGON_DAMAGED_BROWNIMG              int = 21
	HEXAGON_GREY_DARKIMG                  int = 22
	HEXAGON_GREY_GREENIMG                 int = 23
	HEXAGON_GREY_REDIMG                   int = 24
	HEXAGON_GREYIMG                       int = 25
	MINIMAP_ARROW_AIMG                    int = 26
	MINIMAP_ARROW_BIMG                    int = 27
	MINIMAP_ARROW_CIMG                    int = 28
	MINIMAP_ARROW_DIMG                    int = 29
	MINIMAP_COMPASS_FUTURE_EIMG           int = 30
	MINIMAP_COMPASS_FUTURE_NIMG           int = 31
	MINIMAP_COMPASS_FUTURE_SIMG           int = 32
	MINIMAP_COMPASS_FUTURE_WIMG           int = 33
	MINIMAP_COMPASS_TOON_EIMG             int = 34
	MINIMAP_COMPASS_TOON_NIMG             int = 35
	MINIMAP_COMPASS_TOON_SIMG             int = 36
	MINIMAP_COMPASS_TOON_WIMG             int = 37
	MINIMAP_ICON_EXCLAMATION_REDIMG       int = 38
	MINIMAP_ICON_EXCLAMATION_WHITEIMG     int = 39
	MINIMAP_ICON_EXCLAMATION_YELLOWIMG    int = 40
	MINIMAP_ICON_JEWEL_REDIMG             int = 41
	MINIMAP_ICON_JEWEL_WHITEIMG           int = 42
	MINIMAP_ICON_JEWEL_YELLOWIMG          int = 43
	MINIMAP_ICON_STAR_REDIMG              int = 44
	MINIMAP_ICON_STAR_WHITEIMG            int = 45
	MINIMAP_ICON_STAR_YELLOWIMG           int = 46
	MINIMAP_RING_BROWN_DETAILIMG          int = 47
	MINIMAP_RING_BROWNIMG                 int = 48
	MINIMAP_RING_GREY_DETAILIMG           int = 49
	MINIMAP_RING_GREYIMG                  int = 50
	MINIMAP_RING_WHITEIMG                 int = 51
	PANEL_BORDER_BROWN_DETAILIMG          int = 52
	PANEL_BORDER_BROWNIMG                 int = 53
	PANEL_BORDER_GREY_DETAILIMG           int = 54
	PANEL_BORDER_GREYIMG                  int = 55
	PANEL_BROWN_ARROWS_DARK_DETAILIMG     int = 56
	PANEL_BROWN_ARROWS_DARKIMG            int = 57
	PANEL_BROWN_ARROWS_DETAILIMG          int = 58
	PANEL_BROWN_ARROWSIMG                 int = 59
	PANEL_BROWN_CORNERS_AIMG              int = 60
	PANEL_BROWN_CORNERS_BIMG              int = 61
	PANEL_BROWN_DAMAGED_DARKIMG           int = 62
	PANEL_BROWN_DAMAGEDIMG                int = 63
	PANEL_BROWN_DARK_CORNERS_AIMG         int = 64
	PANEL_BROWN_DARK_CORNERS_BIMG         int = 65
	PANEL_BROWN_DARKIMG                   int = 66
	PANEL_BROWNIMG                        int = 67
	PANEL_GREY_BLUEIMG                    int = 68
	PANEL_GREY_BOLTS_BLUEIMG              int = 69
	PANEL_GREY_BOLTS_DARKIMG              int = 70
	PANEL_GREY_BOLTS_DETAIL_AIMG          int = 71
	PANEL_GREY_BOLTS_DETAIL_BIMG          int = 72
	PANEL_GREY_BOLTS_GREENIMG             int = 73
	PANEL_GREY_BOLTS_REDIMG               int = 74
	PANEL_GREY_BOLTSIMG                   int = 75
	PANEL_GREY_DARKIMG                    int = 76
	PANEL_GREY_GREENIMG                   int = 77
	PANEL_GREY_REDIMG                     int = 78
	PANEL_GREYIMG                         int = 79
	PANEL_GRID_BLUEPRINTIMG               int = 80
	PANEL_GRID_PAPERIMG                   int = 81
	PATTERN_DIAGONAL_GREY_LARGEIMG        int = 82
	PATTERN_DIAGONAL_GREY_SMALLIMG        int = 83
	PATTERN_DIAGONAL_RED_LARGEIMG         int = 84
	PATTERN_DIAGONAL_RED_SMALLIMG         int = 85
	PATTERN_DIAGONAL_TRANSPARENT_LARGEIMG int = 86
	PATTERN_DIAGONAL_TRANSPARENT_SMALLIMG int = 87
	PATTERN_GRID_BLUEPRINTIMG             int = 88
	PATTERN_GRID_PAPERIMG                 int = 89
	PROGRESS_BLUE_BORDERIMG               int = 90
	PROGRESS_BLUE_SMALL_BORDERIMG         int = 91
	PROGRESS_BLUE_SMALLIMG                int = 92
	PROGRESS_BLUEIMG                      int = 93
	PROGRESS_GREEN_BORDERIMG              int = 94
	PROGRESS_GREEN_SMALL_BORDERIMG        int = 95
	PROGRESS_GREEN_SMALLIMG               int = 96
	PROGRESS_GREENIMG                     int = 97
	PROGRESS_RED_BORDERIMG                int = 98
	PROGRESS_RED_SMALL_BORDERIMG          int = 99
	PROGRESS_RED_SMALLIMG                 int = 100
	PROGRESS_REDIMG                       int = 101
	PROGRESS_TRANSPARENT_SMALLIMG         int = 102
	PROGRESS_TRANSPARENTIMG               int = 103
	PROGRESS_WHITE_BORDERIMG              int = 104
	PROGRESS_WHITE_SMALL_BORDERIMG        int = 105
	PROGRESS_WHITE_SMALLIMG               int = 106
	PROGRESS_WHITEIMG                     int = 107
	ROUND_BROWN_DARKIMG                   int = 108
	ROUND_BROWNIMG                        int = 109
	ROUND_DAMAGED_BROWN_DARKIMG           int = 110
	ROUND_DAMAGED_BROWNIMG                int = 111
	ROUND_GREY_DARKIMG                    int = 112
	ROUND_GREY_DETAILED_GREENIMG          int = 113
	ROUND_GREY_DETAILED_REDIMG            int = 114
	ROUND_GREYIMG                         int = 115
	SCROLLBAR_BROWN_SMALLIMG              int = 116
	SCROLLBAR_BROWNIMG                    int = 117
	SCROLLBAR_FUTURE_GREY_SMALLIMG        int = 118
	SCROLLBAR_FUTURE_GREYIMG              int = 119
	SCROLLBAR_FUTURE_RED_SMALLIMG         int = 120
	SCROLLBAR_FUTURE_REDIMG               int = 121
	SCROLLBAR_FUTURE_TRANSPARENT_SMALLIMG int = 122
	SCROLLBAR_FUTURE_TRANSPARENTIMG       int = 123
	SCROLLBAR_GREY_SMALLIMG               int = 124
	SCROLLBAR_GREYIMG                     int = 125
	SCROLLBAR_TRANSPARENT_SMALLIMG        int = 126
	SCROLLBAR_TRANSPARENTIMG              int = 127
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
