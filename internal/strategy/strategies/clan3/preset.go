package clan3

import (
	"image"
	"image/color"
	"star-map-tool/internal/detector"
	"star-map-tool/internal/game"
	"star-map-tool/internal/pkg/utils"
)

var (
	// 小地图 - 副本入口才有的紫色标记
	MainArea  = []int{62, 74, 133, 147}
	MainColor = []color.RGBA{{115, 25, 214, 0}, {150, 255, 255, 0}}

	// 右下角 - 匹配进入/进入副本按钮
	DungeonQueueArea  = []int{985, 710, 1250, 755}
	DungeonQueueColor = []color.RGBA{{0, 0, 135, 0}, {225, 225, 255, 0}}

	// 左上角 - 骷髅标
	DungeonReadyArea  = []int{140, 50, 164, 68}
	DungeonReadyColor = []color.RGBA{{0, 0, 190, 0}, {225, 225, 255, 0}}

	// 右上角 - 副本时间
	DungeonRunningArea  = []int{1194, 52, 1250, 67}
	DungeonRunningColor = []color.RGBA{{0, 0, 155, 0}, {225, 225, 255, 0}}

	// 中下 - 角色血条
	PlayerHealthArea  = []int{471, 751, 808, 773}
	PlayerHealthColor = []color.RGBA{{0, 190, 255, 0}, {80, 225, 255, 0}}

	// 中上 - 红色血条
	BossHealth      = []int{494, 51, 799, 72}
	BossHealthColor = []color.RGBA{{0, 236, 244, 0}, {25, 255, 255, 0}}

	// 中下 - 结算界面下一步按钮
	NextArea  = []int{535, 697, 727, 736}
	NextColor = []color.RGBA{{0, 0, 220, 0}, {0, 0, 255, 0}}

	// 中上 - 击败最后一波怪后进入Boss房间的条件识别
	BossConditionArea  = []int{494, 240, 800, 260}
	BossConditionColor = []color.RGBA{{22, 110, 106, 0}, {45, 180, 255, 0}}

	// 右下 - 复活标志(亮)
	RebirthLightArea  = []int{1104, 686, 1143, 718}
	RebirthLightColor = []color.RGBA{{0, 10, 210, 0}, {24, 50, 255, 0}}
)

// 获取在地下城入口的证明标志
func GetMainArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(MainArea[0], MainArea[1], MainArea[2], MainArea[3])
	img, _ := game.GetScreenshotMatRGB(MainArea[0], MainArea[1], w, h)
	defer img.Close()

	// 此区域逻辑上可获取1个紫色框体区域
	params := detector.NewColorDetectParam(img, MainColor[0], MainColor[1], 120)
	return colorDetector.Detect(params)
}

// 获取匹配进入/进入副本按钮标志
func GetDungeonQueueArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(DungeonQueueArea[0], DungeonQueueArea[1], DungeonQueueArea[2], DungeonQueueArea[3])
	img, _ := game.GetScreenshotMatRGB(DungeonQueueArea[0], DungeonQueueArea[1], w, h)
	defer img.Close()

	// 此区域逻辑上可获取2个灰色框体区域
	params := detector.NewColorDetectParam(img, DungeonQueueColor[0], DungeonQueueColor[1], 120)
	return colorDetector.Detect(params)
}

// 获取副本退出按钮标志
func GetDungeonExitArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(DungeonReadyArea[0], DungeonReadyArea[1], DungeonReadyArea[2], DungeonReadyArea[3])
	img, _ := game.GetScreenshotMatRGB(DungeonReadyArea[0], DungeonReadyArea[1], w, h)
	defer img.Close()

	params := detector.NewColorDetectParam(img, DungeonReadyColor[0], DungeonReadyColor[1], 50)
	return colorDetector.Detect(params)
}

// 获取副本进行中的标志
func GetDungeonRunningArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(DungeonRunningArea[0], DungeonRunningArea[1], DungeonRunningArea[2], DungeonRunningArea[3])
	img, _ := game.GetScreenshotMatRGB(DungeonRunningArea[0], DungeonRunningArea[1], w, h)
	defer img.Close()

	params := detector.NewColorDetectParam(img, DungeonRunningColor[0], DungeonRunningColor[1], 40)
	return colorDetector.Detect(params)
}

// 获取玩家血条标志
func GetPlayerHealthArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(PlayerHealthArea[0], PlayerHealthArea[1], PlayerHealthArea[2], PlayerHealthArea[3])
	img, _ := game.GetScreenshotMatRGB(PlayerHealthArea[0], PlayerHealthArea[1], w, h)
	defer img.Close()

	params := detector.NewColorDetectParam(img, PlayerHealthColor[0], PlayerHealthColor[1], 5)
	return colorDetector.Detect(params)
}

// 获取Boss红色血条
func GetBossHealth(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(BossHealth[0], BossHealth[1], BossHealth[2], BossHealth[3])
	img, _ := game.GetScreenshotMatRGB(BossHealth[0], BossHealth[1], w, h)
	defer img.Close()

	param := detector.NewColorDetectParam(img, BossHealthColor[0], BossHealthColor[1], 1)
	return colorDetector.Detect(param)
}

// 获取结算画面下一步按钮标志
func GetNextArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(NextArea[0], NextArea[1], NextArea[2], NextArea[3])
	img, _ := game.GetScreenshotMatRGB(NextArea[0], NextArea[1], w, h)
	defer img.Close()

	// 中间下方 - 下一步 (由于是白灰色的按钮，HSV只取高明度)
	param := detector.NewColorDetectParam(img, NextColor[0], NextColor[1], 300)
	return colorDetector.Detect(param)
}

// 获取最后一波怪被击败的标志
func GetBossConditionArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(BossConditionArea[0], BossConditionArea[1], BossConditionArea[2], BossConditionArea[3])
	img, _ := game.GetScreenshotMatRGB(BossConditionArea[0], BossConditionArea[1], w, h)
	defer img.Close()

	param := detector.NewColorDetectParam(img, BossConditionColor[0], BossConditionColor[1], 800)
	return colorDetector.Detect(param)
}

// 获取重生标志
func GetRebirthLightArea(game game.Game, colorDetector detector.ColorDetector) ([]image.Rectangle, []float64, bool) {
	w, h := utils.GetRectSize(RebirthLightArea[0], RebirthLightArea[1], RebirthLightArea[2], RebirthLightArea[3])
	img, _ := game.GetScreenshotMatRGB(RebirthLightArea[0], RebirthLightArea[1], w, h)
	defer img.Close()

	param := detector.NewColorDetectParam(img, RebirthLightColor[0], RebirthLightColor[1], 40)
	return colorDetector.Detect(param)
}
