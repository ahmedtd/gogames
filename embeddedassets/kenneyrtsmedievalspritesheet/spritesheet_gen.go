package kenneyrtsmedievalspritesheet

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

//go:embed medievalspritesheet.textproto
var textprotoBytes []byte

//go:embed medievalRTS_spritesheet@2.png
var texturePNG []byte

const (
	TREE_TALL                  int = 0
	TREE_CONIFER               int = 1
	TREE_CONIFER_LARGE         int = 2
	STUMP                      int = 3
	LOG                        int = 4
	ROCK_SMALL                 int = 5
	ROCK_LARGE                 int = 6
	ROCK_PILE                  int = 7
	ROCK_PILE_LARGE            int = 8
	ROCK_PILE_LARGE_COPPER     int = 9
	ROCK_PILE_LARGE_GOLD       int = 10
	BUSH                       int = 11
	BROWN_ROCK_SMALL           int = 12
	BROWN_ROCK_LARGE           int = 13
	BROWN_ROCK_PILE            int = 14
	BROWN_ROCK_PILE_LARGE      int = 15
	BROWN_ROCK_PILE_LARGE_IRON int = 16
	BROWN_ROCK_PILE_LARGE_GOLD int = 17
	BUSH_BERRIES               int = 18
	FIRE                       int = 19
	TREE_SMALL                 int = 20
	RED_ROOF_TOWER_TOP         int = 21
	CASTLE_TOP                 int = 22
	HOUSE1                     int = 23
	CHURCH_BASE                int = 24
	STONE_TOWER_BASE           int = 25
	CASTLE_BASE                int = 26
	STORE_ADDON                int = 27
	TOWER_NARROWING_MID        int = 28
	HOUSE_LARGE                int = 29
	STUCCO_TOWER_TOP           int = 30
	STUCCO_TOWER_BASE          int = 31
	FOUNTAIN                   int = 32
	WINDMILLBLADE1             int = 33
	WINDMILLBLADE2             int = 34
	WINDMILLBLADE3             int = 35
	SHED                       int = 36
	HOUSE2                     int = 37
	HOUSE3                     int = 38
	BARN                       int = 39
	HOUSE4                     int = 40
	HOUSE6                     int = 41
	STALL                      int = 42
	CLOCKTOP                   int = 43
	TILE_SAND1                 int = 44
	TILE_SAND2                 int = 45
	TILE_GRASS_ROAD_0DEG       int = 46
	TILE_GRASS_ROAD_90DEG      int = 47
	TILE_GRASS_CROSSROAD       int = 48
	TILE_GRASS_TROAD_180DEG    int = 49
	TILE_GRASS_TROAD_0DEG      int = 50
	ROAD_STRAIGHT_0DEG         int = 51
	ROAD_STRAIGHT_90DEG        int = 52
	ROAD_CROSS                 int = 53
	ROAD_T_180DEG              int = 54
	ROAD_T_0DEG                int = 55
	TILE_DIRT1                 int = 56
	TILE_DIRT2                 int = 57
	TILE_ROCK1                 int = 58
	TILE_ROCK2                 int = 59
	MEDIEVALTILE_17            int = 60
	MEDIEVALTILE_18            int = 61
	MEDIEVALTILE_19            int = 62
	MEDIEVALTILE_20            int = 63
	MEDIEVALTILE_21            int = 64
	MEDIEVALTILE_22            int = 65
	MEDIEVALTILE_23            int = 66
	MEDIEVALTILE_24            int = 67
	MEDIEVALTILE_25            int = 68
	MEDIEVALTILE_26            int = 69
	TILE_ICE1                  int = 70
	TILE_ICE2                  int = 71
	TILE_SNOW1                 int = 72
	TILE_SNOW2                 int = 73
	MEDIEVALTILE_31            int = 74
	MEDIEVALTILE_32            int = 75
	MEDIEVALTILE_33            int = 76
	MEDIEVALTILE_34            int = 77
	MEDIEVALTILE_35            int = 78
	MEDIEVALTILE_36            int = 79
	MEDIEVALTILE_37            int = 80
	MEDIEVALTILE_38            int = 81
	MEDIEVALTILE_39            int = 82
	MEDIEVALTILE_40            int = 83
	MEDIEVALTILE_41            int = 84
	MEDIEVALTILE_42            int = 85
	MEDIEVALTILE_43            int = 86
	MEDIEVALTILE_44            int = 87
	MEDIEVALTILE_45            int = 88
	MEDIEVALTILE_46            int = 89
	MEDIEVALTILE_47            int = 90
	MEDIEVALTILE_48            int = 91
	MEDIEVALTILE_49            int = 92
	MEDIEVALTILE_50            int = 93
	MEDIEVALTILE_51            int = 94
	MEDIEVALTILE_52            int = 95
	MEDIEVALTILE_53            int = 96
	MEDIEVALTILE_54            int = 97
	MEDIEVALTILE_55            int = 98
	MEDIEVALTILE_56            int = 99
	TILE_GRASS1                int = 100
	TILE_GRASS2                int = 101
	BLUE_WOMAN                 int = 102
	BLUE_MAN                   int = 103
	BLUE_CLERIC                int = 104
	BLUE_SPEARMAN              int = 105
	BLUE_HALBERDIER            int = 106
	BLUE_KING                  int = 107
	RED_WOMAN                  int = 108
	RED_MAN                    int = 109
	RED_CLERIC                 int = 110
	RED_SPEARMAN               int = 111
	RED_HALBERDIER             int = 112
	RED_KING                   int = 113
	GREEN_WOMAN                int = 114
	GREEN_MAN                  int = 115
	GREEN_CLERIC               int = 116
	GREEN_SPEARMAN             int = 117
	GREEN_HALBERDIER           int = 118
	GREEN_KING                 int = 119
	GREY_WOMAN                 int = 120
	GREY_MAN                   int = 121
	GREY_CLERIC                int = 122
	GREY_SPEARMAN              int = 123
	GREY_HALBERDIER            int = 124
	GREY_KING                  int = 125
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
