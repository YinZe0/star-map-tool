package sheep3

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"
	"star-map-tool/internal/detector"
	"star-map-tool/internal/pkg/script"
	"star-map-tool/internal/pkg/sleeper"
	"star-map-tool/internal/pkg/utils"
	"star-map-tool/internal/strategy"
	"time"

	"github.com/go-vgo/robotgo"
)

// -------------------------------------------------- 应对BOSS战 ----------------------------------------------------

var classes = []string{"BOSS_REST", "KEY_FLOWER", "KEY_MOON", "KEY_LEAF", "KEY_SWORD"}

// -------------------------------------------------- 应对BOSS战（找墙体） ----------------------------------------------------

func GotoWall(s *StrategyImpl, sctx *strategy.StrategyContext, direction int, duration int) bool {
	log.Printf("[%s-%s] 正在前往任务门位置...\n", s.GetName(), s.GetMode())

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(duration)*time.Millisecond)
	flag1 := make(chan int, 1) // 主线程向子线程写入停止执行命令
	flag2 := make(chan int, 1) // 主线程向子线程写入停止执行命令

	go func() {
		w, h := utils.GetRectSize(494, 51, 799, 72)

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-flag1:
				return
			case <-ticker.C:
				mat, _ := sctx.Game.GetScreenshotMatRGB(494, 51, w, h)
				defer mat.Close()

				// 找红色血条
				param := detector.NewColorDetectParam(mat, color.RGBA{0, 236, 244, 0}, color.RGBA{25, 255, 255, 0}, 300)
				if _, _, ok := s.colorDetector.Detect(param); !ok {
					continue
				}
				log.Printf("[%s-%s] 场地交互已完成,恢复对Boss的攻击\n", s.GetName(), s.GetMode())
				cancel() // 主动告诉上级线程应该停止
				return
			}
		}
	}()

	go func() {
		script.MoveSide(direction, 5_000, 1, flag2)
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				log.Printf("[%s-%s] 场地交互已超时,即将P出副本\n", s.GetName(), s.GetMode())
				flag1 <- 0 // 超时需要额外通知停止dnn循环检测
			}
			flag2 <- 0
			return errors.Is(ctx.Err(), context.Canceled)
		case <-ticker.C:
			if !s.IsEnable() {
				flag1 <- 0
				flag2 <- 0
				return false
			}
		}
	}
}

func findDirectionOfWall(s *StrategyImpl, sctx *strategy.StrategyContext) (int, bool) {
	x, y := robotgo.Location()

	img, _ := sctx.Game.GetScreenshotMatRGB()
	defer img.Close()

	param := detector.NewColorDetectParam(img, color.RGBA{130, 90, 136, 0}, color.RGBA{149, 252, 210, 0}, 300)
	wallRect, _, ok := s.colorDetector.Detect(param)

	if ok {
		log.Printf("[%s-%s] 检测墙体位置成功\n", s.GetName(), s.GetMode())
	} else {
		log.Printf("[%s-%s] 检测墙体位置失败\n", s.GetName(), s.GetMode())
		return 999, false
	}
	wall := wallRect[0]

	// 计算高度
	height := wall.Max.Y - wall.Min.Y

	// 计算方位
	center := utils.GetCenter(wall)
	angle := utils.GetAngle(image.Point{X: x, Y: y}, center, 14.2)
	log.Printf("[%s-%s] 检测墙体高度为:%d 角度:%d\n", s.GetName(), s.GetMode(), height, angle)
	script.ChangeCameraAngleForX(x, y, angle, 3.24)

	direction := 1
	if angle < 0 {
		direction = -1
	}

	switch direction {
	case -1:
		log.Printf("[%s-%s] 检测到任务墙体在左侧\n", s.GetName(), s.GetMode())
	default:
		log.Printf("[%s-%s] 检测到任务墙体在右侧\n", s.GetName(), s.GetMode())
	}
	return direction, ok
}

// -------------------------------------------------- 应对BOSS战（找钥匙） ----------------------------------------------------

func GotoTaskKey(s *StrategyImpl, sctx *strategy.StrategyContext, duration int) bool {
	log.Printf("[%s-%s] 正在前往任务钥匙位置...\n", s.GetName(), s.GetMode())

	direction := sctx.Attrs["KeyDirection"].(int)
	bossKeyClassId := sctx.Attrs["BossKeyClassId"].(int) // 候选值实是 1、2、3
	if direction == 0 {                                  // 在身边就不用动
		return true
	}

	x, y := robotgo.Location()
	script.ChangeCameraAngleForY(x, y, 65, 7.8)
	script.Scroll(10, "up")
	script.Scroll(7, "down") // 控制视角去识别武器

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(duration)*time.Millisecond)
	flag1 := make(chan int, 1) // 主线程向子线程写入停止执行命令
	flag2 := make(chan int, 1) // 主线程向子线程写入停止执行命令

	go func() {
		targetList := []int{1, 2, 3}
		targetList = append(targetList[:bossKeyClassId-1], targetList[bossKeyClassId:]...)
		if ok := findTaskKey(s, sctx, targetList); ok { // 立刻执行一次
			cancel() // 主动告诉上级线程应该停止
			return
		}

		ticker := time.NewTicker(20 * time.Millisecond) // 这里上点压力，执行到这里如果错过就太亏了
		defer ticker.Stop()
		for {
			select {
			case <-flag1:
				return
			case <-ticker.C:
				if ok := findTaskKey(s, sctx, targetList); !ok {
					continue
				}
				cancel() // 主动告诉上级线程应该停止
				return
			}
		}
	}()

	go func() {
		script.MoveSide(direction, 5_000, 0, flag2) // 跑太快会错过检测
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				log.Printf("[%s-%s] 钥匙定位已超时,即将P出副本\n", s.GetName(), s.GetMode())
				flag1 <- 0 // 超时需要额外通知停止dnn循环检测
			}
			flag2 <- 0
			if errors.Is(ctx.Err(), context.Canceled) {
				script.Scroll(10, "up")
				script.Scroll(5, "down")
				script.ChangeCameraAngleForY(x, y, -65, 7.8)
			}
			return errors.Is(ctx.Err(), context.Canceled)
		case <-ticker.C:
			if !s.IsEnable() {
				flag1 <- 0
				flag2 <- 0
				return false
			}
		}
	}
}

func findTaskKey(s *StrategyImpl, sctx *strategy.StrategyContext, targetList []int) bool {
	// 截图、dnn识别目标
	mat, _ := sctx.Game.GetScreenshotMatRGB()
	defer mat.Close()

	param := detector.NewDNNDetectParam(mat, 0.5, 0.45)
	_, _, ok := s.dnnDetector.Detect(param, targetList...)
	return ok
}

func findDirectionOfTaskKey(s *StrategyImpl, sctx *strategy.StrategyContext) (int, int, error) {
	x, y := robotgo.Location()

	script.Scroll(20, "up")
	sleeper.Sleep(50)
	script.Scroll(5, "down")
	sleeper.Sleep(50)
	script.ChangeCameraAngleForY(x, y, 8, 7.8) // 镜头向下一点，避免人物遮挡BOSS（可能因此无法识别boss屁股，可以增加训练或者不改y轴，让x轴错开一点角度）

	// 获取钥匙的方向（不论判断为左或右，都有可能就在自己身边）
	direction, bossKeyClassId, err := findDirectionOfTaskKey0(s, sctx)
	if err != nil {
		log.Printf("[%s-%s] 检测到识别任务钥匙发生异常: %s\n", s.GetName(), s.GetMode(), err.Error())
		return -1, -1, err
	}
	return direction, bossKeyClassId, nil
}

func findDirectionOfTaskKey0(s *StrategyImpl, sctx *strategy.StrategyContext) (int, int, error) {
	_, bossKeyClassId, err := findBossKey(s, sctx)
	if err != nil {
		return -1, -1, err
	}
	log.Printf("[%s-%s] 检测到BOSS头顶钥匙为 %s\n", s.GetName(), s.GetMode(), classes[bossKeyClassId])
	sleeper.Sleep(500)
	x, y := robotgo.Location()

	direction := -2 // -2:未命中 -1:左边 0:身后 1:右边

	// 转身180看看有没有
	script.ChangeCameraAngleForX(x, y, -180, 3.24)

	img, _ := sctx.Game.GetScreenshotMatRGB()
	defer img.Close()
	param := detector.NewDNNDetectParam(img, 0.5, 0.45)
	_, keyClassIdList, _ := s.dnnDetector.Detect(param, 1, 2, 3)
	if len(keyClassIdList) > 0 {
		log.Printf("[%s-%s] 已转向到:180 识别到钥匙数:%d 类值:%s\n", s.GetName(), s.GetMode(), len(keyClassIdList), fmt.Sprint(keyClassIdList))
	} else {
		log.Printf("[%s-%s] 已转向到:180 识别到钥匙数:%d\n", s.GetName(), s.GetMode(), len(keyClassIdList))
	}

	// 先回正
	script.ChangeCameraAngleForX(x, y, 180, 3.24)
	sleeper.Sleep(100)

	if len(keyClassIdList) == 1 && keyClassIdList[0] != bossKeyClassId {
		direction = 0
	} else {
		var list = []int{-30, 60} // 被遮挡了 或 模型原因没识别出来，被遮挡指左侧钥匙在boss背后的扇形范围内，那么右边的钥匙一定在身边（转身看看）

		j := 0
		for i := range list {
			script.ChangeCameraAngleForX(x, y, list[i], 3.24)
			j++

			img, _ := sctx.Game.GetScreenshotMatRGB()
			defer img.Close()

			param := detector.NewDNNDetectParam(img, 0.5, 0.45)
			_, keyClassIdList, _ := s.dnnDetector.Detect(param, 1, 2, 3)
			if len(keyClassIdList) > 0 {
				log.Printf("[%s-%s] 已转向到:%d 识别到钥匙数:%d 类值:%s\n", s.GetName(), s.GetMode(), list[i], len(keyClassIdList), fmt.Sprint(keyClassIdList))
			} else {
				log.Printf("[%s-%s] 已转向到:%d 识别到钥匙数:%d\n", s.GetName(), s.GetMode(), list[i], len(keyClassIdList))
			}

			if len(keyClassIdList) != 2 {
				continue
			}
			if i == 0 && keyClassIdList[0] == keyClassIdList[1] {
				direction = 1
				break
			} else if i == 0 && keyClassIdList[0] != keyClassIdList[1] {
				direction = -1
				break
			} else if i == 1 && keyClassIdList[0] == keyClassIdList[1] {
				direction = -1
				break
			} else if i == 1 && keyClassIdList[0] != keyClassIdList[1] {
				direction = 1
				break
			}
		}
		// 回正视角
		switch j {
		case 1:
			script.ChangeCameraAngleForX(x, y, 30, 3.24)
		case 2:
			script.ChangeCameraAngleForX(x, y, -30, 3.24)
		}
	}

	// TOD: 考虑在自己身后
	switch direction {
	case -2:
		return -2, -1, errors.New("未能发现有效的钥匙")
	case -1:
		log.Printf("[%s-%s] 检测到任务钥匙在左侧\n", s.GetName(), s.GetMode())
	case 0:
		log.Printf("[%s-%s] 检测到任务钥匙在身边\n", s.GetName(), s.GetMode())
	default:
		log.Printf("[%s-%s] 检测到任务钥匙在右侧\n", s.GetName(), s.GetMode())
	}
	return direction, bossKeyClassId, nil

}

// -------------------------------------------------- 应对BOSS战（找BOSS） ----------------------------------------------------

func findAndFaceTheBoss(s *StrategyImpl, sctx *strategy.StrategyContext, angle int) bool {
	x, y := robotgo.Location()

	var boss image.Rectangle
	ok, _ := utils.NewTicker(20*time.Second, 200*time.Millisecond, func() (bool, error) {
		rect, err := findBoss(s, sctx)
		if err != nil {
			script.ChangeCameraAngleForX(x, y, angle, 3.24) // 找不到就转向
			return false, nil
		}
		boss = rect
		return true, nil // 未能识别前进方向
	}, true)
	if !ok {
		log.Printf("[%s-%s] 未能定位到BOSS\n", s.GetName(), s.GetMode())
		return false
	}
	log.Printf("[%s-%s] 已成功定位BOSS\n", s.GetName(), s.GetMode())

	// 一次定位：尝试面向BOSS
	center := utils.GetCenter(boss)
	angle = utils.GetAngle(image.Point{X: x, Y: y}, center, 14.2)
	script.ChangeCameraAngleForX(x, y, angle, 3.24)
	sleeper.Sleep(200)

	// 二次定位（因为第一次可能只看到一个角，转的角度并非正对boss）
	boss, err := findBoss(s, sctx)
	if err == nil {
		center = utils.GetCenter(boss)
		angle = utils.GetAngle(image.Point{X: x, Y: y}, center, 14.2)
		script.ChangeCameraAngleForX(x, y, angle, 3.24)
	}
	log.Printf("[%s-%s] 已转正面向BOSS\n", s.GetName(), s.GetMode())
	sleeper.Sleep(200)
	return true
}

func findBoss(s *StrategyImpl, sctx *strategy.StrategyContext) (image.Rectangle, error) {
	// 要求已经面对BOSS
	mat, _ := sctx.Game.GetScreenshotMatRGB()
	defer mat.Close()

	param := detector.NewDNNDetectParam(mat, 0.5, 0.1)
	rectList, classIdList, ok := s.dnnDetector.Detect(param, 0)
	if !ok {
		return image.Rectangle{}, errors.New("无法识别Boss")
	}

	for i := range classIdList {
		if classIdList[i] == 0 {
			return rectList[i], nil
		}
	}
	return image.Rectangle{}, errors.New("无法识别Boss")
}

func findBossKey(s *StrategyImpl, sctx *strategy.StrategyContext) (image.Rectangle, int, error) {
	// 要求已经面对BOSS
	mat, _ := sctx.Game.GetScreenshotMatRGB()
	defer mat.Close()

	param := detector.NewDNNDetectParam(mat, 0.5, 0.45)
	rectList, classIdList, ok := s.dnnDetector.Detect(param, 1, 2, 3)
	if !ok {
		return image.Rectangle{}, 0, errors.New("无法识别Boss钥匙")
	}

	var key image.Rectangle
	var keyClassId int

	// 位于屏幕最高处的那个key视为boss头顶的key，如果不行就得换成 findSwordKey 的根据x距离最接近的查找方法
	minY := 9999
	for i := range rectList {
		if classIdList[i] != 1 && classIdList[i] != 2 && classIdList[i] != 3 {
			continue
		}
		rect := rectList[i]
		if rect.Min.Y >= minY { // 屏幕左上角是(0, 0)，因此rect.y > minY，代表rect在屏幕下面的位置
			continue
		}
		minY = rect.Min.Y
		key = rect
		keyClassId = classIdList[i]
	}
	return key, keyClassId, nil
}
