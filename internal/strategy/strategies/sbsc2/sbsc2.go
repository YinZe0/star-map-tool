package sbsc2

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"
	"path/filepath"
	"star-map-tool/internal/detector"
	"star-map-tool/internal/pkg/paths"
	"star-map-tool/internal/pkg/script"
	"star-map-tool/internal/pkg/sleeper"
	"star-map-tool/internal/pkg/utils"
	"star-map-tool/internal/strategy"
	"sync/atomic"
	"time"

	"github.com/go-vgo/robotgo"
)

// 策略算是单例的，上下文每次执行都是新的
type Sbsc2Strategy struct {
	enable  int32
	context *strategy.StrategyContext

	colorDetector detector.ColorDetector
	dnnDetector   detector.DNNDetector
	script        script.Script
}

func NewSbsc2Strategy() strategy.Strategy {
	return &Sbsc2Strategy{
		enable: 1,
	}
}

func (s *Sbsc2Strategy) GetName() string {
	return "衰败深处"
}

func (s *Sbsc2Strategy) GetMode() string {
	return "困难"
}

func (s *Sbsc2Strategy) IsEnable() bool {
	return atomic.LoadInt32(&s.enable) == 1
}

func (s *Sbsc2Strategy) Enable() {
	atomic.StoreInt32(&s.enable, 1)
}

func (s *Sbsc2Strategy) Disable() {
	atomic.StoreInt32(&s.enable, 0)
}

// -------------------------------------------------- 策略主逻辑 ----------------------------------------------------

func (s *Sbsc2Strategy) Init() {
	s.colorDetector = detector.NewColorDetector()
	s.dnnDetector = detector.NewDNNDetector(paths.GetAbsolutePath(filepath.Join("assets", "models", "sbsc", "best.onnx")))
	s.script = script.NewDefaultScript()
}

func (s *Sbsc2Strategy) Execute(sctx *strategy.StrategyContext, data interface{}) bool {
	s.context = sctx // 每次的策略上下文都是新的
	s.Enable()

	go s.runDeatchCheck()

	var operationList []script.Operation
	operationList = append(operationList, s.goToDungeon()...)
	operationList = append(operationList, s.startDungeon()...)
	operationList = append(operationList, s.handleScence1()...)
	operationList = append(operationList, s.handleScence2()...)
	operationList = append(operationList, s.handleScence3()...)
	operationList = append(operationList, s.handleScence4()...)
	operationList = append(operationList, s.handleScence5()...)
	operationList = append(operationList, s.handleBossScence()...)

	return s.runOperationList(operationList)
}

func (s *Sbsc2Strategy) Abort(sign string) {
	s.Disable()
}

func (s *Sbsc2Strategy) runOperationList(list []script.Operation) bool {
	for _, op := range list {
		if !s.IsEnable() {
			s.exitDungeon()
			return false
		}
		ok := op()
		if !ok {
			s.exitDungeon()
			return false
		}
	}
	return true
}

func (s *Sbsc2Strategy) runDeatchCheck() {
	// 死亡处理：一般只检测途中，死亡后直接退出（如果不退出需要更复杂的操作去识别、修正行为）
	running := false
	for {
		flag := atomic.LoadInt32(&s.context.DeathCheckFlag)
		if running && flag == 0 {
			// 逻辑内关闭死亡检测 - 通常到达BOSS战才会关闭
			return
		} else if !s.IsEnable() {
			// 有其他逻辑中断策略执行，停止死亡检测
			return
		} else if flag == 0 {
			sleeper.Sleep(200)
			continue
		}
		if !running {
			running = true
		}

		// 中间下面 - 绿色
		w, h := utils.GetRectSize(471, 751, 638, 773)
		img, _ := s.context.Game.GetScreenshotMatRGB(471, 751, w, h)
		defer img.Close()

		params := detector.NewColorDetectParam(img, color.RGBA{65, 0, 255, 0}, color.RGBA{80, 225, 255, 0}, 10)
		_, _, ok := s.colorDetector.Detect(params)
		flag = atomic.LoadInt32(&s.context.DeathCheckFlag)
		if !ok && flag == 1 { // 已死亡
			log.Printf("[%s-%s] 未检测到玩家血条,认定为已死亡(即将执行P出逻辑)\n", s.GetName(), s.GetMode())
			running = false
			s.Disable() // 交给主线程去退出对局
			return
		}
		sleeper.Sleep(200)
	}
}

func (s *Sbsc2Strategy) exitDungeon() {
	robotgo.Click() // 有可能小月卡弹框
	sleeper.Sleep(200)

	// 直接p，死亡状态按p是无效的，如果能退就退了
	robotgo.KeyTap("p")
	robotgo.MoveClick(794, 579)

	// p不出去就点死亡时出现的退出按钮
	robotgo.MoveClick(1179, 67)
	robotgo.MoveClick(794, 579)

	log.Printf("[%s-%s] 已执行副本退出逻辑\n", s.GetName(), s.GetMode())
}

func (s *Sbsc2Strategy) handleBossScence() []script.Operation {
	return []script.Operation{
		s.script.Wait(5000),

		// 检测是否进入boss房间
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			w, h := utils.GetRectSize(140, 50, 164, 68)
			ok := utils.NewTicker(20*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("超时")
				}
				img, _ := sctx.Game.GetScreenshotMatRGB(140, 50, w, h)
				defer img.Close()

				// 左上角 - 骷髅标
				params := detector.NewColorDetectParam(img, color.RGBA{0, 0, 190, 0}, color.RGBA{225, 225, 255, 0}, 90)
				_, _, ok := s.colorDetector.Detect(params)
				return ok, nil
			}, true)
			if !ok {
				log.Printf("[%s-%s] 检测到未进入Boss场景\n", s.GetName(), s.GetMode())
			} else {
				log.Printf("[%s-%s] 检测到已进入Boss场景\n", s.GetName(), s.GetMode())
			}
			return ok, nil
		}, func() *strategy.StrategyContext { return s.context }),

		// 开怪
		s.script.Move([]string{"w", "shift"}, 1500),
		s.script.TapOnce("h"),
		s.script.Wait(5_000),
		s.script.TapOnce("h"),
		s.script.Move([]string{"s", "shift"}, 1500),

		// 持续检查是否进入二阶段
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			w, h := utils.GetRectSize(494, 51, 799, 72)
			utils.NewTicker(4*time.Minute, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("超时")
				}
				mat, _ := sctx.Game.GetScreenshotMatRGB(494, 51, w, h)
				defer mat.Close()

				// 正中上方 - 灰色血条
				param := detector.NewColorDetectParam(mat, color.RGBA{0, 0, 159, 0}, color.RGBA{0, 0, 174, 0}, 300)
				_, _, ok := s.colorDetector.Detect(param)
				return ok, nil
			}, true)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Wait(1000),

		// 通过转向找到目标钥匙
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			if ok := findAndFaceTheBoss(s, sctx, -45); !ok {
				return false, errors.New("定位BOSS失败")
			}
			direction, bossClassId, err := findDirectionOfTaskKey(s, sctx)
			if err == nil {
				sctx.Attrs["KeyDirection"] = direction
				sctx.Attrs["BossKeyClassId"] = bossClassId // boss头顶的钥匙
				return true, nil
			}
			return false, nil
		}, func() *strategy.StrategyContext { return s.context }),
		// 前往目标钥匙
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			ok := gotoTaskKey(s, sctx, 15_000)
			return ok, nil
		}, func() *strategy.StrategyContext { return s.context }),

		// 等待钥匙被AI击败后，找到并前往门的方向
		s.script.Wait(8_000),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			direction := sctx.Attrs["KeyDirection"].(int)
			angle := 45
			if direction == 1 {
				angle = -angle
			}

			if ok := findAndFaceTheBoss(s, sctx, angle); !ok {
				return false, errors.New("定位BOSS失败")
			}
			direction, ok := findDirectionOfWall(s, sctx)
			if !ok {
				return false, errors.New("定位墙体失败")
			}
			ok = gotoWall(s, sctx, direction, 30_000)
			return ok, nil
		}, func() *strategy.StrategyContext { return s.context }),

		// 持续检查是否进入结算阶段
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			w, h := utils.GetRectSize(1103, 683, 1144, 718)
			ok := utils.NewTicker(3*time.Minute, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("超时")
				}
				img, _ := sctx.Game.GetScreenshotMatRGB(1103, 683, w, h)
				defer img.Close()

				// 中间下方 - 下一步
				params := detector.NewColorDetectParam(img, color.RGBA{0, 0, 225, 0}, color.RGBA{225, 225, 255, 0}, 300)
				_, _, ok := s.colorDetector.Detect(params)
				if ok {
					robotgo.MoveClick(635, 715)
					robotgo.MoveClick(935, 735)
				}
				return ok, nil
			}, false)
			return ok, nil
		}, func() *strategy.StrategyContext { return s.context }),
	}
}

func (s *Sbsc2Strategy) handleScence5() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.ExecTask(func(*strategy.StrategyContext) (bool, error) {
			log.Printf("[%s-%s] 执行第5个关卡(特征:胖子)\n", s.GetName(), s.GetMode())
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),

		s.script.Wait(4_000), // 回体力用
		s.script.Move([]string{"s"}, 500),
		s.script.ChangeCameraAngleForX(x, y, 92, 3.24),
		s.script.Move([]string{"w", "shift"}, 5_000),
		s.script.Wait(1000),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			robotgo.Click()
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Wait(400),
		s.script.Move([]string{"d", "shift"}, 50_000), // 这里会被吸走，全凭移动 + ai奶尽可能幸存

		s.script.ChangeCameraAngleForX(x, y, -38, 3.24),
		s.script.Move([]string{"w", "shift"}, 10_000),
		s.script.ChangeCameraAngleForX(x, y, 31, 3.24),
		s.script.Move([]string{"d"}, 800),

		s.script.ExecTask(func(*strategy.StrategyContext) (bool, error) {
			log.Printf("[%s-%s] 正在前往Boss房...\n", s.GetName(), s.GetMode())
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),

		// 在走过场之前关闭死亡检测，不然死亡检测会因为过场动画误判
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			atomic.StoreInt32(&sctx.DeathCheckFlag, 0) // 关闭死亡检测
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Move([]string{"w", "shift"}, 10_000),
	}
}

func (s *Sbsc2Strategy) handleScence4() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.ExecTask(func(*strategy.StrategyContext) (bool, error) {
			log.Printf("[%s-%s] 执行第4个关卡(特征:羊)\n", s.GetName(), s.GetMode())
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),

		s.script.Move([]string{"w", "shift"}, 4000),
		s.script.Wait(5_000),
		s.script.Move([]string{"s"}, 650),
		s.script.ChangeCameraAngleForX(x, y, 60, 3.24),
		s.script.Move([]string{"w"}, 5000),
		s.script.Move([]string{"a", "shift"}, 2000),
		s.script.Wait(10_000),
		s.script.Move([]string{"d"}, 1050),
		s.script.MoveAndOnce([]string{"w", "shift"}, 11_000, func(*strategy.StrategyContext) (bool, error) {
			robotgo.KeyTap("e")
			sleeper.SleepBusyLoop(600)
			robotgo.KeyTap("e")
			sleeper.Sleep(1000)

			// 游戏特性: e之后不跟shift，会变为走路
			robotgo.KeyUp("shift")
			robotgo.KeyDown("shift")
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
	}
	// min 154 215 0  max 255 255 59  可以识别到传送门 有需要可以通过颜色识别并导航
}

func (s *Sbsc2Strategy) handleScence3() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.ExecTask(func(*strategy.StrategyContext) (bool, error) {
			log.Printf("[%s-%s] 执行第3个关卡(特征:姆克)\n", s.GetName(), s.GetMode())
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Move([]string{"w", "shift"}, 5000),

		s.script.ChangeCameraAngleForX(x, y, -52, 3.24),
		s.script.Move([]string{"w", "shift"}, 8000),

		s.script.ChangeCameraAngleForX(x, y, 180, 3.24),
		s.script.Wait(30_000),
		s.script.ChangeCameraAngleForX(x, y, -180, 3.24),
	}
}

func (s *Sbsc2Strategy) handleScence2() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.ExecTask(func(*strategy.StrategyContext) (bool, error) {
			log.Printf("[%s-%s] 执行第2个关卡(特征:羊)\n", s.GetName(), s.GetMode())
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Wait(30_000),

		s.script.Move([]string{"d", "shift"}, 1000),
		s.script.Move([]string{"a"}, 800),
		s.script.Move([]string{"w", "shift"}, 500),
		s.script.Move([]string{"a", "shift"}, 4000),
		s.script.Wait(2000),

		s.script.ChangeCameraAngleForX(x, y, 17, 3.24),
		s.script.Move([]string{"w", "shift"}, 7000),
		s.script.Wait(2000),
		s.script.Move([]string{"s"}, 400),

		s.script.ChangeCameraAngleForX(x, y, 85, 3.24),
		s.script.Move([]string{"w", "shift"}, 3500),
		s.script.Move([]string{"d", "shift"}, 1000),
		s.script.ChangeCameraAngleForX(x, y, -17, 3.24),
	}
}

func (s *Sbsc2Strategy) handleScence1() []script.Operation {
	// 在洞口站着等，然后再到光剑后面站着等
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.ExecTask(func(*strategy.StrategyContext) (bool, error) {
			log.Printf("[%s-%s] 执行第1个关卡(特征:蜥蜴)\n", s.GetName(), s.GetMode())
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Wait(2000),

		s.script.Move([]string{"w", "shift"}, 4000),
		s.script.ChangeCameraAngleForX(x, y, -67, 3.24),
		s.script.Move([]string{"w", "shift"}, 2000),
		s.script.Wait(20_000),

		s.script.Move([]string{"w", "shift"}, 2400),
		s.script.ChangeCameraAngleForX(x, y, -90, 3.24),
		s.script.ChangeCameraAngleForX(x, y, -22, 3.24),
		s.script.MoveAndOnce([]string{"w", "shift"}, 4500, func(*strategy.StrategyContext) (bool, error) {
			robotgo.KeyTap("e")
			sleeper.SleepBusyLoop(600)
			robotgo.KeyTap("e")
			sleeper.Sleep(200)

			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Wait(25_000),

		s.script.Move([]string{"s"}, 1000),
		s.script.ChangeCameraAngleForY(x, y, 45, 7.8),
		s.script.MoveAndKeep([]string{}, 40_000, 300, func(sctx *strategy.StrategyContext) (bool, error) {
			startTime := sctx.Attrs["_scence1_start_time"].(time.Time)
			d := sctx.Attrs["_scence1_direction"].(int32)
			if d == 0 {
				d = 1 // 只有这一个线程在访问这个属性，可以这么搞
				robotgo.KeyDown("ctrl")
				robotgo.KeyDown("w")
			}

			w, h := utils.GetRectSize(288, 192, 958, 721)
			mat, _ := sctx.Game.GetScreenshotMatRGB(288, 192, w, h)
			defer mat.Close()

			// 从颜色区间内寻找
			mincolor := color.RGBA{R: 20, G: 40, B: 200, A: 0}
			maxcolor := color.RGBA{R: 40, G: 90, B: 255, A: 0}
			param := detector.NewColorDetectParam(mat, mincolor, maxcolor, 60)
			_, _, ok := s.colorDetector.Detect(param)
			if time.Since(startTime) < 12*time.Second { // 15秒的机会去不停的找剑，找不到就继续往后走，找到就恢复往前走
				if !ok {
					if d == 1 {
						d = 2
						robotgo.KeyUp("w")
						robotgo.KeyUp("ctrl")
					}
					robotgo.KeyDown("s")
					sleeper.Sleep(1500)
					robotgo.KeyUp("s")
				} else {
					d = 0
				}
			} else {
				if !ok {
					sleeper.Sleep(1800)
					robotgo.KeyUp("w")
					robotgo.KeyUp("ctrl")
					log.Printf("[%s-%s] 光墙开启成功\n", s.GetName(), s.GetMode())
					return true, nil
				}
			}
			return false, nil
		}, func() *strategy.StrategyContext {
			s.context.Attrs["_scence1_direction"] = int32(0)
			s.context.Attrs["_scence1_start_time"] = time.Now()
			return s.context
		}),
		s.script.ChangeCameraAngleForY(x, y, -45, 7.8),
		s.script.ChangeCameraAngleForX(x, y, 2, 3.24),
	}
}

func (s *Sbsc2Strategy) startDungeon() []script.Operation {
	return []script.Operation{
		s.script.Wait(10 * 1000), // 等待加载
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			// 检查是否进地图了
			w, h := utils.GetRectSize(128, 180, 207, 201)
			ok := utils.NewTicker(15*time.Second, 1*time.Second, func() (bool, error) {
				img, _ := sctx.Game.GetScreenshotMatRGB(128, 180, w, h)
				defer img.Close()

				// 左上角 - 退出副本
				params := detector.NewColorDetectParam(img, color.RGBA{0, 0, 155, 0}, color.RGBA{225, 225, 255, 0}, 50)
				_, _, ok := s.colorDetector.Detect(params)
				return ok, nil
			}, true)
			if !ok {
				log.Printf("[%s-%s] 检测到未进入地下城\n", s.GetName(), s.GetMode())
				return false, nil
			} else {
				log.Printf("[%s-%s] 检测到已进入地下城\n", s.GetName(), s.GetMode())
				return true, nil
			}
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Move([]string{"w"}, 1200),
		s.script.TapOnce("f"),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			log.Printf("[%s-%s] 正在开启地下城...\n", s.GetName(), s.GetMode())
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Wait(200),
		s.script.MouseMoveClick(810, 665),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			// 检查副本是否开启成功
			w, h := utils.GetRectSize(1194, 52, 1250, 67)
			ok := utils.NewTicker(15*time.Second, 1*time.Second, func() (bool, error) {
				img, _ := sctx.Game.GetScreenshotMatRGB(1194, 52, w, h)
				defer img.Close()

				// 右侧交互设备 - 开启副本
				params := detector.NewColorDetectParam(img, color.RGBA{0, 0, 155, 0}, color.RGBA{225, 225, 255, 0}, 40)
				_, _, ok := s.colorDetector.Detect(params)
				return ok, nil
			}, true)
			if !ok {
				log.Printf("[%s-%s] 检测到未开启地下城\n", s.GetName(), s.GetMode())
			} else {
				log.Printf("[%s-%s] 检测到已开启地下城\n", s.GetName(), s.GetMode())
			}
			return ok, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.ExecTask(func(scxt *strategy.StrategyContext) (bool, error) {
			atomic.StoreInt32(&s.context.DeathCheckFlag, 1) // 开启死亡检测
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
	}
}

func (s *Sbsc2Strategy) goToDungeon() []script.Operation {
	return []script.Operation{
		s.script.TapOnce("f"),
		s.script.Wait(200),
		s.script.MouseMoveClick(124, 208),
		s.script.Wait(200),
		s.script.MouseMoveClick(1000, 690),
		s.script.Wait(200),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			// 检查有没有按钮（有可能轮之间间隔太短，角色还没回主城呢）
			w, h := utils.GetRectSize(985, 710, 1250, 755)
			img, _ := sctx.Game.GetScreenshotMatRGB(985, 710, w, h)
			defer img.Close()

			// 右下角 - 进入副本
			params := detector.NewColorDetectParam(img, color.RGBA{0, 0, 135, 0}, color.RGBA{225, 225, 255, 0}, 4000)
			if rectList, _, ok := s.colorDetector.Detect(params); !ok || len(rectList) != 2 {
				return false, nil
			}
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.MouseMoveClick(1185, 735),
		s.script.MouseMove(0, 0),
		s.script.Wait(3 * 1000),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			// 这里要检测是否开启排队（游戏Bug可能造成根本没进行排队）
			w, h := utils.GetRectSize(985, 710, 1250, 755)
			img, _ := sctx.Game.GetScreenshotMatRGB(985, 710, w, h)
			defer img.Close()

			// 右下角 - 进入副本
			params := detector.NewColorDetectParam(img, color.RGBA{0, 0, 135, 0}, color.RGBA{225, 225, 255, 0}, 200)
			if rectList, _, ok := s.colorDetector.Detect(params); ok && len(rectList) == 2 {
				log.Printf("[%s-%s] 检测到异常队伍,正在执行退出队伍操作...\n", s.GetName(), s.GetMode())
				robotgo.KeyTap("esc")
				script.HandleAbnormalTeam(sctx.Game)

				s.runOperationList(s.goToDungeon())
			}
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			log.Printf("[%s-%s] 正在进入地下城...", s.GetName(), s.GetMode())
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
	}
}

// -------------------------------------------------- 应对BOSS战 ----------------------------------------------------

var classes = []string{"BOSS_REST", "KEY_FLOWER", "KEY_MOON", "KEY_LEAF", "KEY_SWORD"}

// -------------------------------------------------- 应对BOSS战（找墙体） ----------------------------------------------------

func gotoWall(s *Sbsc2Strategy, sctx *strategy.StrategyContext, direction int, duration int) bool {
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
				cancel() // 主动告诉上级线程应该停止
				return
			}
		}
	}()

	go func() {
		script.MoveSide(direction, 5_000, 0, flag2)
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
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

func findDirectionOfWall(s *Sbsc2Strategy, sctx *strategy.StrategyContext) (int, bool) {
	x, y := robotgo.Location()

	img, _ := sctx.Game.GetScreenshotMatRGB()
	defer img.Close()

	param := detector.NewColorDetectParam(img, color.RGBA{130, 90, 136, 0}, color.RGBA{149, 252, 210, 0}, 300)
	wallRect, _, ok := s.colorDetector.Detect(param)

	if ok {
		log.Printf("[%s-%s] 检测墙体位置成功\n", s.GetName(), s.GetMode())
	} else {
		log.Printf("[%s-%s] 检测墙体位置失败\n", s.GetName(), s.GetMode())
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

func gotoTaskKey(s *Sbsc2Strategy, sctx *strategy.StrategyContext, duration int) bool {
	log.Printf("[%s-%s] 正在前往任务钥匙位置...\n", s.GetName(), s.GetMode())

	direction := sctx.Attrs["KeyDirection"].(int)
	bossKeyClassId := sctx.Attrs["BossKeyClassId"].(int) // 候选值实是 1、2、3
	if direction == 0 {                                  // 在身边就不用动
		return true
	}

	x, y := robotgo.Location()
	script.ChangeCameraAngleForY(x, y, 65, 7.8)
	script.Scroll(10, "up")
	script.Scroll(5, "down") // 控制视角去识别武器

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

		ticker := time.NewTicker(150 * time.Millisecond)
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
		script.MoveSide(direction, 5_000, 1, flag2)
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				log.Printf("[%s-%s] 未能在时限内定位到钥匙", s.GetName(), s.GetMode())
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

func findTaskKey(s *Sbsc2Strategy, sctx *strategy.StrategyContext, targetList []int) bool {
	// 截图、dnn识别目标
	mat, _ := sctx.Game.GetScreenshotMatRGB()
	defer mat.Close()

	param := detector.NewDNNDetectParam(mat, 0.6, 0.45)
	_, _, ok := s.dnnDetector.Detect(param, targetList...)
	return ok
}

func findDirectionOfTaskKey(s *Sbsc2Strategy, sctx *strategy.StrategyContext) (int, int, error) {
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

func findDirectionOfTaskKey0(s *Sbsc2Strategy, sctx *strategy.StrategyContext) (int, int, error) {
	_, bossKeyClassId, err := findBossKey(s, sctx)
	if err != nil {
		return -1, -1, err
	}
	log.Printf("[%s-%s] 检测到BOSS头顶钥匙为 %s\n", s.GetName(), s.GetMode(), classes[bossKeyClassId])
	sleeper.Sleep(500)
	x, y := robotgo.Location()

	direction := -2 // -2:未命中 -1:左边 0:身后 1:右边

	// 转身180看看有没有
	script.ChangeCameraAngleForX(x, y, -90, 3.24)
	script.ChangeCameraAngleForX(x, y, -90, 3.24)

	img, _ := sctx.Game.GetScreenshotMatRGB()
	defer img.Close()
	param := detector.NewDNNDetectParam(img, 0.6, 0.45)
	_, keyClassIdList, _ := s.dnnDetector.Detect(param, 1, 2, 3)
	if len(keyClassIdList) > 0 {
		log.Printf("[%s-%s] 已转向到:180 识别到钥匙数:%d 类值:%s\n", s.GetName(), s.GetMode(), len(keyClassIdList), fmt.Sprint(keyClassIdList))
	} else {
		log.Printf("[%s-%s] 已转向到:180 识别到钥匙数:%d\n", s.GetName(), s.GetMode(), len(keyClassIdList))
	}

	// 先回正
	script.ChangeCameraAngleForX(x, y, 90, 3.24)
	script.ChangeCameraAngleForX(x, y, 90, 3.24)
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

			param := detector.NewDNNDetectParam(img, 0.6, 0.45)
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

func findAndFaceTheBoss(s *Sbsc2Strategy, sctx *strategy.StrategyContext, angle int) bool {
	x, y := robotgo.Location()

	var boss image.Rectangle
	ok := utils.NewTicker(20*time.Second, 200*time.Millisecond, func() (bool, error) {
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

func findBoss(s *Sbsc2Strategy, sctx *strategy.StrategyContext) (image.Rectangle, error) {
	// 要求已经面对BOSS
	mat, _ := sctx.Game.GetScreenshotMatRGB()
	defer mat.Close()

	param := detector.NewDNNDetectParam(mat, 0.6, 0.1)
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

func findBossKey(s *Sbsc2Strategy, sctx *strategy.StrategyContext) (image.Rectangle, int, error) {
	// 要求已经面对BOSS
	mat, _ := sctx.Game.GetScreenshotMatRGB()
	defer mat.Close()

	param := detector.NewDNNDetectParam(mat, 0.6, 0.45)
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
