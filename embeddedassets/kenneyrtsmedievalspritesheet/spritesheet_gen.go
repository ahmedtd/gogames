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

const TREE_TALL int = 0

const TREE_CONIFER int = 1

const TREE_CONIFER_LARGE int = 2

const STUMP int = 3

const LOG int = 4

const MEDIEVALENVIRONMENT_06 int = 5

const MEDIEVALENVIRONMENT_07 int = 6

const MEDIEVALENVIRONMENT_08 int = 7

const MEDIEVALENVIRONMENT_09 int = 8

const MEDIEVALENVIRONMENT_10 int = 9

const MEDIEVALENVIRONMENT_11 int = 10

const MEDIEVALENVIRONMENT_12 int = 11

const MEDIEVALENVIRONMENT_13 int = 12

const MEDIEVALENVIRONMENT_14 int = 13

const MEDIEVALENVIRONMENT_15 int = 14

const MEDIEVALENVIRONMENT_16 int = 15

const MEDIEVALENVIRONMENT_17 int = 16

const MEDIEVALENVIRONMENT_18 int = 17

const MEDIEVALENVIRONMENT_19 int = 18

const MEDIEVALENVIRONMENT_20 int = 19

const MEDIEVALENVIRONMENT_21 int = 20

const MEDIEVALSTRUCTURE_01 int = 21

const MEDIEVALSTRUCTURE_02 int = 22

const MEDIEVALSTRUCTURE_03 int = 23

const MEDIEVALSTRUCTURE_04 int = 24

const MEDIEVALSTRUCTURE_05 int = 25

const MEDIEVALSTRUCTURE_06 int = 26

const MEDIEVALSTRUCTURE_07 int = 27

const MEDIEVALSTRUCTURE_08 int = 28

const MEDIEVALSTRUCTURE_09 int = 29

const MEDIEVALSTRUCTURE_10 int = 30

const MEDIEVALSTRUCTURE_11 int = 31

const MEDIEVALSTRUCTURE_12 int = 32

const MEDIEVALSTRUCTURE_13 int = 33

const MEDIEVALSTRUCTURE_14 int = 34

const MEDIEVALSTRUCTURE_15 int = 35

const MEDIEVALSTRUCTURE_16 int = 36

const MEDIEVALSTRUCTURE_17 int = 37

const MEDIEVALSTRUCTURE_18 int = 38

const MEDIEVALSTRUCTURE_19 int = 39

const MEDIEVALSTRUCTURE_20 int = 40

const MEDIEVALSTRUCTURE_21 int = 41

const MEDIEVALSTRUCTURE_22 int = 42

const MEDIEVALSTRUCTURE_23 int = 43

const TILE_SAND int = 44

const MEDIEVALTILE_02 int = 45

const MEDIEVALTILE_03 int = 46

const MEDIEVALTILE_04 int = 47

const MEDIEVALTILE_05 int = 48

const MEDIEVALTILE_06 int = 49

const MEDIEVALTILE_07 int = 50

const MEDIEVALTILE_08 int = 51

const MEDIEVALTILE_09 int = 52

const MEDIEVALTILE_10 int = 53

const MEDIEVALTILE_11 int = 54

const MEDIEVALTILE_12 int = 55

const MEDIEVALTILE_13 int = 56

const MEDIEVALTILE_14 int = 57

const MEDIEVALTILE_15 int = 58

const MEDIEVALTILE_16 int = 59

const MEDIEVALTILE_17 int = 60

const MEDIEVALTILE_18 int = 61

const MEDIEVALTILE_19 int = 62

const MEDIEVALTILE_20 int = 63

const MEDIEVALTILE_21 int = 64

const MEDIEVALTILE_22 int = 65

const MEDIEVALTILE_23 int = 66

const MEDIEVALTILE_24 int = 67

const MEDIEVALTILE_25 int = 68

const MEDIEVALTILE_26 int = 69

const MEDIEVALTILE_27 int = 70

const MEDIEVALTILE_28 int = 71

const MEDIEVALTILE_29 int = 72

const MEDIEVALTILE_30 int = 73

const MEDIEVALTILE_31 int = 74

const MEDIEVALTILE_32 int = 75

const MEDIEVALTILE_33 int = 76

const MEDIEVALTILE_34 int = 77

const MEDIEVALTILE_35 int = 78

const MEDIEVALTILE_36 int = 79

const MEDIEVALTILE_37 int = 80

const MEDIEVALTILE_38 int = 81

const MEDIEVALTILE_39 int = 82

const MEDIEVALTILE_40 int = 83

const MEDIEVALTILE_41 int = 84

const MEDIEVALTILE_42 int = 85

const MEDIEVALTILE_43 int = 86

const MEDIEVALTILE_44 int = 87

const MEDIEVALTILE_45 int = 88

const MEDIEVALTILE_46 int = 89

const MEDIEVALTILE_47 int = 90

const MEDIEVALTILE_48 int = 91

const MEDIEVALTILE_49 int = 92

const MEDIEVALTILE_50 int = 93

const MEDIEVALTILE_51 int = 94

const MEDIEVALTILE_52 int = 95

const MEDIEVALTILE_53 int = 96

const MEDIEVALTILE_54 int = 97

const MEDIEVALTILE_55 int = 98

const MEDIEVALTILE_56 int = 99

const MEDIEVALTILE_57 int = 100

const MEDIEVALTILE_58 int = 101

const BLUE_WOMAN int = 102

const BLUE_MAN int = 103

const BLUE_CLERIC int = 104

const BLUE_SPEARMAN int = 105

const BLUE_HALBERDIER int = 106

const BLUE_KING int = 107

const RED_WOMAN int = 108

const RED_MAN int = 109

const RED_CLERIC int = 110

const RED_SPEARMAN int = 111

const RED_HALBERDIER int = 112

const RED_KING int = 113

const GREEN_WOMAN int = 114

const GREEN_MAN int = 115

const GREEN_CLERIC int = 116

const GREEN_SPEARMAN int = 117

const GREEN_HALBERDIER int = 118

const GREEN_KING int = 119

const GREY_WOMAN int = 120

const GREY_MAN int = 121

const GREY_CLERIC int = 122

const GREY_SPEARMAN int = 123

const GREY_HALBERDIER int = 124

const GREY_KING int = 125

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
