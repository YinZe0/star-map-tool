package clan3

import (
	"image"
	"image/color"
	"star-map-tool/internal/detector"
	"star-map-tool/internal/game"
	"star-map-tool/internal/pkg/utils"
)

var (
	// 屏幕中间 - 地面阵法花纹
	PatternArea  = []int{640, 150, 750, 530}
	PatternColor = []color.RGBA{{85, 105, 213, 0}, {255, 255, 255, 0}}

	// 屏幕中间 - 地面阵法花纹（使用后）
	PatternUsedArea  = []int{640, 150, 750, 530}
	PatternUsedColor = []color.RGBA{{105, 30, 213, 0}, {255, 115, 255, 0}}

	// 中上 - 必杀剑技能提示
	SwordArea  = []int{450, 220, 850, 300}
	SwordColor = []color.RGBA{{22, 110, 106, 0}, {45, 180, 255, 0}}
)

func GetPatternArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(PatternArea[0], PatternArea[1], PatternArea[2], PatternArea[3])
	img, _ := game.GetScreenshotMatRGB(PatternArea[0], PatternArea[1], w, h)
	defer img.Close()

	param := detector.NewColorDetectParam(img, PatternColor[0], PatternColor[1], 600)
	return colorDetector.Detect(param)
}

func GetPatternUsedArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(PatternUsedArea[0], PatternUsedArea[1], PatternUsedArea[2], PatternUsedArea[3])
	img, _ := game.GetScreenshotMatRGB(PatternUsedArea[0], PatternUsedArea[1], w, h)
	defer img.Close()

	param := detector.NewColorDetectParam(img, PatternColor[0], PatternColor[1], 600)
	return colorDetector.Detect(param)
}

func GetSwordArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(SwordArea[0], SwordArea[1], SwordArea[2], SwordArea[3])
	img, _ := game.GetScreenshotMatRGB(SwordArea[0], SwordArea[1], w, h)
	defer img.Close()

	param := detector.NewColorDetectParam(img, SwordColor[0], SwordColor[1], 100)
	return colorDetector.Detect(param)
}
