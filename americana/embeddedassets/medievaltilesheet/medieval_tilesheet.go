package medievaltilesheet

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"

	"shellworld/lib/resource/tilesheet"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed medieval_tilesheet.png
var data []byte

type TileType int

const (
	TileGrass1 TileType = iota
	TileGrass2
	TileSand1
	TileSand2
	TileGrassRoadEW
	TileGrassRoadNS
	TileGrassRoadNESW
	TileGrassRoadESW
	TileGrassRoadNEW
	RoadNS
	RoadEW
	RoadNESW
	RoadESW
	RoadNEW
	BuildingChurchTop
	BuildingTowerTop
	BuildingCastleTop
	BuildingArcheryRangeTop
	TileDirt1
	TileDirt2
	TileStone1
	TileStone2
	TileGrassRoadES
	TileGrassRoadSW
	TileGrassRoadW
	TileGrassRoadNSW
	TileGrassRoadNES
	RoadES
	RoadSW
	RoadW
	RoadNSW
	RoadNES
	BuildingChurchBase
	BuildingTowerBase
	BuildingCastleBase
	BuildingArcheryRangeBase
	TileWater1
	TileWater2
	TileSnow1
	TileSnow2
	TileGrassRoadNE
	TileGrassRoadNW
	TileGrassRoadE
	TileGrassRoadN
	TileGrassRoadS
	RoadNE
	RoadNW
	RoadE
	RoadN
	RoadS
	BuildingMidPiece   // Not sure what this is supposed to be
	BuildingOtherPiece // Not sure what this is supposed to be
	BuildingWindmillTop
	BuildingWindmillBase
	TileGrassForestLeafy1
	TileGrassForestLeafy2
	TileGrassForestLeafy3
	TileGrassForestLeafy4
	TreeLeafySmall
	TreeLeafyLarge
	TreeConiferSmall
	TreeConiferLarge
	Stump
	Log
	Statue
	WomanBlue
	ManBlue
	PriestBlue
	SpearmanBlue
	HalberdmanBlue
	KingBlue
	WindmillVanes1
	TileGrassConifer1
	TileGrassConifer2
	TileGrassConifer3
	TileGrassConifer4
	GrayRockSmall
	GrayRockMedium
	GrayRockPileSmall
	GrayRockPileLarge
	GrayRockPileLargeBrownDots
	GrayRockPileLargeGoldDots
	Bush
	WomanOrange
	ManOrange
	PriestOrange
	SpearmanOrange
	HalberdmanOrange
	KingOrange
	WindmillVanes2
	TileGrassStump1
	TileGrassStump2
	TileGrassStump3
	TileGrassStump4
	BrownRockSmall
	BrownRockLarge
	BrownRockPileSmall
	BrownRockPileLarge
	BrownRockPileLargeGrayDots
	BrownRockPileLargeGoldDots
	BerryBush
	WomanGreen
	ManGreen
	PriestGreen
	SpearmanGreen
	HalberdmanGreen
	KingGreen
	WindmillVanes3
	TileGrassSmallFarmBare
	TileGrassSmallFarmGrain
	TileGrassLargeFarmBare
	TileGrassLargeFarmGrain
	BuildingOpenShed
	BuildingHouse1
	BuildingHouse2
	BuildingBarn
	BuildingHouseChimney
	BuildinghouseShed
	BuildingStall
	WomanGray
	ManGray
	PriestGray
	SpearmanGray
	HalberdmanGray
	KingGray
	Fire
)

func Load() (*tilesheet.T, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("while decoding PNG: %w", err)
	}

	ebimg := ebiten.NewImageFromImage(img)

	return &tilesheet.T{
		Image: ebimg,

		NumTilesX:   18,
		NumTilesY:   7,
		TileSizeX:   64,
		TileSizeY:   64,
		TileMarginX: 32,
		TileMarginY: 32,

		UnitsPerPixel: 1 / 64.0,
	}, nil
}
