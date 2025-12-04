package robot2

import (
	"image"
	"image/color"
	"star-map-tool/internal/detector"
	"star-map-tool/internal/game"
	"star-map-tool/internal/pkg/utils"
)

var (
	// 全屏 - 能量球
	SphereArea  = []int{0, 70, 1280, 596}
	SphereColor = []color.RGBA{{90, 50, 230, 0}, {100, 73, 255, 0}}
)

func GetSphereArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(SphereArea[0], SphereArea[1], SphereArea[2], SphereArea[3])
	img, _ := game.GetScreenshotMatRGB(SphereArea[0], SphereArea[1], w, h)
	defer img.Close()

	param := detector.NewColorDetectParam(img, SphereColor[0], SphereColor[1], 40)
	return colorDetector.Detect(param)
}
