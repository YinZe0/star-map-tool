package farm60

import (
	"image"
	"image/color"
	"star-map-tool/internal/detector"
	"star-map-tool/internal/game"
	"star-map-tool/internal/pkg/utils"
)

var (
	FarmDoorArea  = []int{}
	FarmDoorColor = []color.RGBA{{135, 135, 150, 0}, {170, 255, 255, 0}}

	FarmStateArea  = []int{284, 132, 385, 207}
	FarmStateColor = []color.RGBA{{0, 0, 150, 0}, {190, 50, 255, 0}}
)

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
