package sheep2

import (
	"image"
	"image/color"
	"star-map-tool/internal/detector"
	"star-map-tool/internal/game"
	"star-map-tool/internal/pkg/utils"
)

var (
	// 中间 - 开门的光剑
	Sword1Area  = []int{288, 192, 958, 721}
	Sword1Color = []color.RGBA{{20, 40, 200, 0}, {40, 90, 255, 0}}

	// 中上 - BOSS站姿时角的识别
	BossArea       = []int{520, 60, 770, 330}
	BossRangeColor = []color.RGBA{{167, 157, 153, 0}, {255, 255, 255, 0}}
)

// 获取第一个关卡的开门钥匙标志
func GetSwordKey1Area(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(Sword1Area[0], Sword1Area[1], Sword1Area[2], Sword1Area[3])
	mat, _ := game.GetScreenshotMatRGB(Sword1Area[0], Sword1Area[1], w, h)
	defer mat.Close()

	param := detector.NewColorDetectParam(mat, Sword1Color[0], Sword1Color[1], 60)
	return colorDetector.Detect(param)
}

// 获取Boss标志（由于没有对Boss站姿进行训练，只能通过识别角的颜色来进行处理）
func GetBossArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(BossArea[0], BossArea[1], BossArea[2], BossArea[3])
	img, _ := game.GetScreenshotMatRGB(BossArea[0], BossArea[1], w, h)
	defer img.Close()

	param := detector.NewColorDetectParam(img, BossRangeColor[0], BossRangeColor[1], 20)
	return colorDetector.Detect(param)
}
