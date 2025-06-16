package kenneyrtsmedievalspritesheet

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"
	"shellworld/lib/resource/spritesheet"

	"github.com/hajimehoshi/ebiten/v2"
)

var TREE_TALL = &spritesheet.AtlasEntry{
	X:       1029,
	Y:       636,
	Width:   38,
	Height:  92,
	CenterX: 19.5,
	CenterY: 80,
}

var MEDIEVALENVIRONMENT_02 = &spritesheet.AtlasEntry{
	X:       1030,
	Y:       728,
	Width:   37,
	Height:  66,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_03 = &spritesheet.AtlasEntry{
	X:       1023,
	Y:       0,
	Width:   53,
	Height:  96,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_04 = &spritesheet.AtlasEntry{
	X:       838,
	Y:       736,
	Width:   28,
	Height:  30,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_05 = &spritesheet.AtlasEntry{
	X:       766,
	Y:       736,
	Width:   72,
	Height:  30,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_06 = &spritesheet.AtlasEntry{
	X:       804,
	Y:       603,
	Width:   36,
	Height:  32,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_07 = &spritesheet.AtlasEntry{
	X:       976,
	Y:       963,
	Width:   64,
	Height:  56,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_08 = &spritesheet.AtlasEntry{
	X:       950,
	Y:       535,
	Width:   76,
	Height:  64,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_09 = &spritesheet.AtlasEntry{
	X:       950,
	Y:       465,
	Width:   76,
	Height:  70,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_10 = &spritesheet.AtlasEntry{
	X:       950,
	Y:       395,
	Width:   76,
	Height:  70,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_11 = &spritesheet.AtlasEntry{
	X:       950,
	Y:       325,
	Width:   76,
	Height:  70,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_12 = &spritesheet.AtlasEntry{
	X:       903,
	Y:       255,
	Width:   40,
	Height:  46,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_13 = &spritesheet.AtlasEntry{
	X:       768,
	Y:       603,
	Width:   36,
	Height:  32,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_14 = &spritesheet.AtlasEntry{
	X:       951,
	Y:       190,
	Width:   64,
	Height:  56,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_15 = &spritesheet.AtlasEntry{
	X:       900,
	Y:       950,
	Width:   76,
	Height:  64,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_16 = &spritesheet.AtlasEntry{
	X:       900,
	Y:       1014,
	Width:   76,
	Height:  70,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_17 = &spritesheet.AtlasEntry{
	X:       951,
	Y:       120,
	Width:   76,
	Height:  70,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_18 = &spritesheet.AtlasEntry{
	X:       950,
	Y:       255,
	Width:   76,
	Height:  70,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_19 = &spritesheet.AtlasEntry{
	X:       863,
	Y:       255,
	Width:   40,
	Height:  47,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_20 = &spritesheet.AtlasEntry{
	X:       1026,
	Y:       318,
	Width:   43,
	Height:  96,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALENVIRONMENT_21 = &spritesheet.AtlasEntry{
	X:       1076,
	Y:       0,
	Width:   28,
	Height:  64,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_01 = &spritesheet.AtlasEntry{
	X:       957,
	Y:       699,
	Width:   72,
	Height:  84,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_02 = &spritesheet.AtlasEntry{
	X:       384,
	Y:       1024,
	Width:   120,
	Height:  76,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_03 = &spritesheet.AtlasEntry{
	X:       608,
	Y:       1024,
	Width:   104,
	Height:  68,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_04 = &spritesheet.AtlasEntry{
	X:       812,
	Y:       1013,
	Width:   88,
	Height:  72,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_05 = &spritesheet.AtlasEntry{
	X:       862,
	Y:       399,
	Width:   88,
	Height:  84,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_06 = &spritesheet.AtlasEntry{
	X:       128,
	Y:       1024,
	Width:   120,
	Height:  76,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_07 = &spritesheet.AtlasEntry{
	X:       504,
	Y:       1024,
	Width:   104,
	Height:  68,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_08 = &spritesheet.AtlasEntry{
	X:       959,
	Y:       0,
	Width:   64,
	Height:  80,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_09 = &spritesheet.AtlasEntry{
	X:       760,
	Y:       893,
	Width:   112,
	Height:  120,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_10 = &spritesheet.AtlasEntry{
	X:       862,
	Y:       555,
	Width:   88,
	Height:  80,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_11 = &spritesheet.AtlasEntry{
	X:       863,
	Y:       175,
	Width:   88,
	Height:  80,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_12 = &spritesheet.AtlasEntry{
	X:       1026,
	Y:       514,
	Width:   40,
	Height:  72,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_13 = &spritesheet.AtlasEntry{
	X:       768,
	Y:       0,
	Width:   103,
	Height:  103,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_14 = &spritesheet.AtlasEntry{
	X:       640,
	Y:       768,
	Width:   125,
	Height:  125,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_15 = &spritesheet.AtlasEntry{
	X:       640,
	Y:       640,
	Width:   126,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_16 = &spritesheet.AtlasEntry{
	X:       869,
	Y:       736,
	Width:   88,
	Height:  120,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_17 = &spritesheet.AtlasEntry{
	X:       871,
	Y:       0,
	Width:   88,
	Height:  120,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_18 = &spritesheet.AtlasEntry{
	X:       862,
	Y:       303,
	Width:   88,
	Height:  96,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_19 = &spritesheet.AtlasEntry{
	X:       765,
	Y:       768,
	Width:   104,
	Height:  120,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_20 = &spritesheet.AtlasEntry{
	X:       766,
	Y:       640,
	Width:   104,
	Height:  96,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_21 = &spritesheet.AtlasEntry{
	X:       640,
	Y:       893,
	Width:   120,
	Height:  120,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_22 = &spritesheet.AtlasEntry{
	X:       976,
	Y:       883,
	Width:   64,
	Height:  80,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALSTRUCTURE_23 = &spritesheet.AtlasEntry{
	X:       862,
	Y:       483,
	Width:   88,
	Height:  72,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_01 = &spritesheet.AtlasEntry{
	X:       0,
	Y:       640,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_02 = &spritesheet.AtlasEntry{
	X:       0,
	Y:       768,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_03 = &spritesheet.AtlasEntry{
	X:       0,
	Y:       896,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_04 = &spritesheet.AtlasEntry{
	X:       128,
	Y:       128,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_05 = &spritesheet.AtlasEntry{
	X:       128,
	Y:       384,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_06 = &spritesheet.AtlasEntry{
	X:       128,
	Y:       640,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_07 = &spritesheet.AtlasEntry{
	X:       128,
	Y:       768,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_08 = &spritesheet.AtlasEntry{
	X:       1026,
	Y:       190,
	Width:   60,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_09 = &spritesheet.AtlasEntry{
	X:       0,
	Y:       1024,
	Width:   128,
	Height:  60,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_10 = &spritesheet.AtlasEntry{
	X:       256,
	Y:       128,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_11 = &spritesheet.AtlasEntry{
	X:       256,
	Y:       256,
	Width:   128,
	Height:  94,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_12 = &spritesheet.AtlasEntry{
	X:       256,
	Y:       606,
	Width:   128,
	Height:  94,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_13 = &spritesheet.AtlasEntry{
	X:       256,
	Y:       700,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_14 = &spritesheet.AtlasEntry{
	X:       640,
	Y:       0,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_15 = &spritesheet.AtlasEntry{
	X:       640,
	Y:       128,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_16 = &spritesheet.AtlasEntry{
	X:       640,
	Y:       384,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_17 = &spritesheet.AtlasEntry{
	X:       640,
	Y:       512,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_18 = &spritesheet.AtlasEntry{
	X:       384,
	Y:       640,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_19 = &spritesheet.AtlasEntry{
	X:       0,
	Y:       128,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_20 = &spritesheet.AtlasEntry{
	X:       0,
	Y:       0,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_21 = &spritesheet.AtlasEntry{
	X:       384,
	Y:       512,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_22 = &spritesheet.AtlasEntry{
	X:       768,
	Y:       303,
	Width:   94,
	Height:  86,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_23 = &spritesheet.AtlasEntry{
	X:       870,
	Y:       635,
	Width:   86,
	Height:  94,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_24 = &spritesheet.AtlasEntry{
	X:       712,
	Y:       1013,
	Width:   100,
	Height:  72,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_25 = &spritesheet.AtlasEntry{
	X:       768,
	Y:       389,
	Width:   94,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_26 = &spritesheet.AtlasEntry{
	X:       768,
	Y:       175,
	Width:   95,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_27 = &spritesheet.AtlasEntry{
	X:       384,
	Y:       896,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_28 = &spritesheet.AtlasEntry{
	X:       512,
	Y:       640,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_29 = &spritesheet.AtlasEntry{
	X:       0,
	Y:       384,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_30 = &spritesheet.AtlasEntry{
	X:       0,
	Y:       256,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_31 = &spritesheet.AtlasEntry{
	X:       512,
	Y:       128,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_32 = &spritesheet.AtlasEntry{
	X:       512,
	Y:       256,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_33 = &spritesheet.AtlasEntry{
	X:       512,
	Y:       384,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_34 = &spritesheet.AtlasEntry{
	X:       640,
	Y:       256,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_35 = &spritesheet.AtlasEntry{
	X:       512,
	Y:       0,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_36 = &spritesheet.AtlasEntry{
	X:       872,
	Y:       856,
	Width:   86,
	Height:  94,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_37 = &spritesheet.AtlasEntry{
	X:       768,
	Y:       517,
	Width:   94,
	Height:  86,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_38 = &spritesheet.AtlasEntry{
	X:       768,
	Y:       103,
	Width:   100,
	Height:  72,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_39 = &spritesheet.AtlasEntry{
	X:       956,
	Y:       599,
	Width:   72,
	Height:  100,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_40 = &spritesheet.AtlasEntry{
	X:       958,
	Y:       783,
	Width:   72,
	Height:  100,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_41 = &spritesheet.AtlasEntry{
	X:       512,
	Y:       896,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_42 = &spritesheet.AtlasEntry{
	X:       512,
	Y:       768,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_43 = &spritesheet.AtlasEntry{
	X:       256,
	Y:       478,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_44 = &spritesheet.AtlasEntry{
	X:       512,
	Y:       512,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_45 = &spritesheet.AtlasEntry{
	X:       256,
	Y:       350,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_46 = &spritesheet.AtlasEntry{
	X:       256,
	Y:       0,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_47 = &spritesheet.AtlasEntry{
	X:       128,
	Y:       896,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_48 = &spritesheet.AtlasEntry{
	X:       128,
	Y:       512,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_49 = &spritesheet.AtlasEntry{
	X:       128,
	Y:       256,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_50 = &spritesheet.AtlasEntry{
	X:       384,
	Y:       768,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_51 = &spritesheet.AtlasEntry{
	X:       128,
	Y:       0,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_52 = &spritesheet.AtlasEntry{
	X:       0,
	Y:       512,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_53 = &spritesheet.AtlasEntry{
	X:       384,
	Y:       384,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_54 = &spritesheet.AtlasEntry{
	X:       384,
	Y:       256,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_55 = &spritesheet.AtlasEntry{
	X:       384,
	Y:       128,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_56 = &spritesheet.AtlasEntry{
	X:       384,
	Y:       0,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_57 = &spritesheet.AtlasEntry{
	X:       256,
	Y:       956,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALTILE_58 = &spritesheet.AtlasEntry{
	X:       256,
	Y:       828,
	Width:   128,
	Height:  128,
	CenterX: 0,
	CenterY: 0,
}

var BLUE_MAN = &spritesheet.AtlasEntry{
	X:       1067,
	Y:       96,
	Width:   33,
	Height:  48,
	CenterX: 17,
	CenterY: 40,
}

var MEDIEVALUNIT_02 = &spritesheet.AtlasEntry{
	X:       909,
	Y:       120,
	Width:   41,
	Height:  52,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_03 = &spritesheet.AtlasEntry{
	X:       1030,
	Y:       794,
	Width:   36,
	Height:  50,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_04 = &spritesheet.AtlasEntry{
	X:       1026,
	Y:       464,
	Width:   40,
	Height:  50,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_05 = &spritesheet.AtlasEntry{
	X:       1066,
	Y:       794,
	Width:   33,
	Height:  48,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_06 = &spritesheet.AtlasEntry{
	X:       1058,
	Y:       994,
	Width:   33,
	Height:  48,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_07 = &spritesheet.AtlasEntry{
	X:       1058,
	Y:       1042,
	Width:   33,
	Height:  48,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_08 = &spritesheet.AtlasEntry{
	X:       868,
	Y:       120,
	Width:   41,
	Height:  52,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_09 = &spritesheet.AtlasEntry{
	X:       1040,
	Y:       844,
	Width:   36,
	Height:  50,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_10 = &spritesheet.AtlasEntry{
	X:       1027,
	Y:       96,
	Width:   40,
	Height:  50,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_11 = &spritesheet.AtlasEntry{
	X:       1066,
	Y:       462,
	Width:   33,
	Height:  48,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_12 = &spritesheet.AtlasEntry{
	X:       1068,
	Y:       558,
	Width:   33,
	Height:  48,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_13 = &spritesheet.AtlasEntry{
	X:       1066,
	Y:       414,
	Width:   33,
	Height:  48,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_14 = &spritesheet.AtlasEntry{
	X:       1017,
	Y:       1019,
	Width:   41,
	Height:  52,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_15 = &spritesheet.AtlasEntry{
	X:       1040,
	Y:       894,
	Width:   36,
	Height:  50,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_16 = &spritesheet.AtlasEntry{
	X:       1026,
	Y:       414,
	Width:   40,
	Height:  50,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_17 = &spritesheet.AtlasEntry{
	X:       1066,
	Y:       510,
	Width:   33,
	Height:  48,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_18 = &spritesheet.AtlasEntry{
	X:       1067,
	Y:       636,
	Width:   33,
	Height:  48,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_19 = &spritesheet.AtlasEntry{
	X:       1067,
	Y:       684,
	Width:   33,
	Height:  48,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_20 = &spritesheet.AtlasEntry{
	X:       976,
	Y:       1019,
	Width:   41,
	Height:  52,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_21 = &spritesheet.AtlasEntry{
	X:       1040,
	Y:       944,
	Width:   36,
	Height:  50,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_22 = &spritesheet.AtlasEntry{
	X:       1028,
	Y:       586,
	Width:   40,
	Height:  50,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_23 = &spritesheet.AtlasEntry{
	X:       1067,
	Y:       732,
	Width:   33,
	Height:  48,
	CenterX: 0,
	CenterY: 0,
}

var MEDIEVALUNIT_24 = &spritesheet.AtlasEntry{
	X:       1069,
	Y:       318,
	Width:   33,
	Height:  48,
	CenterX: 0,
	CenterY: 0,
}

//go:embed medievalRTS_spritesheet@2.png
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
