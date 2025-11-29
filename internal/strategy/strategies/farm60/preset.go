package farm60

import (
	"image"
	"image/color"
	"star-map-tool/internal/detector"
	"star-map-tool/internal/game"
	"star-map-tool/internal/pkg/utils"
)

var (
	// 中下 - 角色血条
	PlayerHealthArea  = []int{471, 751, 808, 773}
	PlayerHealthColor = []color.RGBA{{0, 190, 255, 0}, {80, 225, 255, 0}}

	// 右下 - 复活标志(亮)
	RebirthLightArea  = []int{1104, 686, 1143, 718}
	RebirthLightColor = []color.RGBA{{0, 10, 210, 0}, {24, 50, 255, 0}}

	FarmDoorArea  = []int{}
	FarmDoorColor = []color.RGBA{{135, 135, 150, 0}, {170, 255, 255, 0}}

	FarmStateArea  = []int{284, 132, 385, 207}
	FarmStateColor = []color.RGBA{{0, 0, 150, 0}, {190, 50, 255, 0}}
)

// 获取玩家血条标志
func GetPlayerHealthArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(PlayerHealthArea[0], PlayerHealthArea[1], PlayerHealthArea[2], PlayerHealthArea[3])
	img, _ := game.GetScreenshotMatRGB(PlayerHealthArea[0], PlayerHealthArea[1], w, h)
	defer img.Close()
	params := detector.NewColorDetectParam(img, PlayerHealthColor[0], PlayerHealthColor[1], 5)
	return colorDetector.Detect(params)
}

// 获取重生标志
func GetRebirthLightArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(RebirthLightArea[0], RebirthLightArea[1], RebirthLightArea[2], RebirthLightArea[3])
	img, _ := game.GetScreenshotMatRGB(RebirthLightArea[0], RebirthLightArea[1], w, h)
	defer img.Close()

	param := detector.NewColorDetectParam(img, RebirthLightColor[0], RebirthLightColor[1], 40)
	return colorDetector.Detect(param)
}

func GetFarmDoorArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	img, _ := game.GetScreenshotMatRGB()
	defer img.Close()

	param := detector.NewColorDetectParam(img, FarmDoorColor[0], FarmDoorColor[1], 200)
	return colorDetector.Detect(param)
}

func GetFarmStateArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(FarmStateArea[0], FarmStateArea[1], FarmStateArea[2], FarmStateArea[3])
	img, _ := game.GetScreenshotMatRGB(FarmStateArea[0], FarmStateArea[1], w, h)
	defer img.Close()

	param := detector.NewColorDetectParam(img, FarmStateColor[0], FarmStateColor[1], 100)
	return colorDetector.Detect(param)
}
