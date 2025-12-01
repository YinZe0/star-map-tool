package robot2

import (
	"errors"
	"image"
	"log"
	"math"
	"star-map-tool/internal/detector"
	"star-map-tool/internal/pkg/script"
	"star-map-tool/internal/pkg/sleeper"
	"star-map-tool/internal/pkg/utils"
	"star-map-tool/internal/strategy"
	"sync/atomic"
	"time"

	"github.com/go-vgo/robotgo"
)

// 策略算是单例的，上下文每次执行都是新的
type StrategyImpl struct {
	enable  int32
	context *strategy.StrategyContext

	colorDetector detector.ColorDetector
	script        script.Script
}

func NewStrategyImpl() strategy.Strategy {
	return &StrategyImpl{
		enable: 1,
	}
}

func (s *StrategyImpl) GetName() string {
	return "无音之都"
}

func (s *StrategyImpl) GetMode() string {
	return "困难"
}

func (s *StrategyImpl) IsEnable() bool {
	return atomic.LoadInt32(&s.enable) == 1
}

func (s *StrategyImpl) Enable() {
	atomic.StoreInt32(&s.enable, 1)
}

func (s *StrategyImpl) Disable(reason int32) {
	// -1: 终止(超时)、-2: 执行失败、-3: 死亡、0: 执行结束、1: 可用
	atomic.StoreInt32(&s.enable, reason)
}

func (s *StrategyImpl) StartDeathCheck(ctx *strategy.StrategyContext) {
	atomic.StoreInt32(&ctx.DeathCheckFlag, 1) // 开启死亡检测
}

func (s *StrategyImpl) StopDeathCheck(ctx *strategy.StrategyContext) {
	atomic.StoreInt32(&ctx.DeathCheckFlag, 0) // 关闭死亡检测
}

func (s *StrategyImpl) Init() {
	s.colorDetector = detector.NewColorDetector()
	s.script = script.NewDefaultScript()
}

func (s *StrategyImpl) Execute(sctx *strategy.StrategyContext, data any) bool {
	s.context = sctx // 每次的策略上下文都是新的
	s.context.Attrs["START_TIME"] = time.Now()
	s.Enable()
	go s.runDeatchCheck()

	var operationList []script.Operation
	// operationList = append(operationList, s.goToDungeon()...)
	// operationList = append(operationList, s.startDungeon()...)
	// operationList = append(operationList, s.handleScence1()...)
	// operationList = append(operationList, s.handleScence2()...)
	// operationList = append(operationList, s.handleScence3()...)
	// operationList = append(operationList, s.handleScence4()...)
	operationList = append(operationList, s.handleBossScence()...)
	return s.run(operationList)
}

func (s *StrategyImpl) Abort(sign string) {
	s.Disable(-1)
}

func (s *StrategyImpl) run(list []script.Operation) bool {
	for _, op := range list {
		if !s.IsEnable() {
			s.exitDungeon()
			return false
		}
		ok := op()
		if !ok {
			s.Disable(-2)
			s.exitDungeon()
			return false
		}
	}
	s.Disable(0) // 让子线程有停止的机会
	return true
}

func (s *StrategyImpl) runDeatchCheck() {
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

		_, _, ok := GetPlayerHealthArea(*s.context.Game, s.colorDetector)
		flag = atomic.LoadInt32(&s.context.DeathCheckFlag)
		if !ok && flag == 1 { // 已死亡
			log.Printf("[%s-%s] 未检测到玩家血条,认定为已死亡(即将执行P出逻辑)\n", s.GetName(), s.GetMode())
			running = false
			s.Disable(-3) // 交给主线程去退出对局
			return
		}
		sleeper.Sleep(200)
	}
}

func (s *StrategyImpl) exitDungeon() {
	enable := atomic.LoadInt32(&s.enable)
	if enable == -2 || enable == -3 {
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
}

func (s *StrategyImpl) handleBossScence() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Wait(6000),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			return utils.NewTicker(3*time.Minute, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetPlayerHealthArea(*sctx.Game, s.colorDetector)
				return ok, nil
			}, false)
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Wait(3000),
		s.script.Log(s.GetName(), s.GetMode(), "执行第Boss关卡"),
		s.script.Move([]string{"w", "shift"}, 2_000),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			return utils.NewTicker(2*time.Minute, 800*time.Millisecond, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetRebirthLightArea(*sctx.Game, s.colorDetector)
				if ok { // 人机打的太慢了
					robotgo.MoveClick(1123, 700)
					sleeper.Sleep(6_000)
				}
				_, _, ok = GetBossGrayHealth(*sctx.Game, s.colorDetector)
				return ok, nil
			}, false)
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Wait(4500),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			var times int32 = 0
			utils.NewTicker(3*time.Minute, 100*time.Millisecond, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				rectList, _, ok := GetSphereArea(*sctx.Game, s.colorDetector)
				if !ok || len(rectList) <= 0 {
					script.ChangeCameraAngleForX(x, y, -50, 3.24)
					sleeper.SleepBusyLoop(500)

					_, _, ok = GetBossHealth(*sctx.Game, s.colorDetector)
					// return times >= 4, nil
					return ok, nil
				}
				if times == 0 {
					robotgo.KeyTap("q")
					sleeper.SleepBusyLoop(400)
				}
				times++

				// 找到最靠近屏幕中心的，位于角色前面的物体
				var target image.Rectangle
				minXDiff := 999
				flag := false
				for _, item := range rectList {
					center := utils.GetCenter(item)
					xdiff := center.X - x
					ydiff := center.Y - y // 过滤掉 y diff >= 0 的（只找面前的, <0）
					if ydiff >= 0 {
						continue
					}
					if math.Abs(float64(xdiff)) < math.Abs(float64(minXDiff)) {
						minXDiff = xdiff
						target = item
						flag = true
					}
				}
				if !flag {
					script.ChangeCameraAngleForX(x, y, -50, 3.24)
					sleeper.SleepBusyLoop(500)
					return false, nil
				}

				rect := target
				center := utils.GetCenter(rect)
				angle := utils.GetAngle(image.Point{X: x, Y: y}, center, 14.2)
				script.ChangeCameraAngleForX(x, y, angle, 3.24)
				sleeper.Sleep(300)
				robotgo.KeyDown("w")
				if math.Abs(float64(angle)) <= 3 {
					sleeper.Sleep(1700)
				} else {
					sleeper.SleepBusyLoop(700)
				}
				robotgo.KeyUp("w")
				return false, nil
			}, false)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			return utils.NewTicker(10*time.Minute, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetRebirthLightArea(*sctx.Game, s.colorDetector)
				if ok { // 人机打的太慢了
					robotgo.MoveClick(1123, 700)
					sleeper.Sleep(6_000)
				}
				// 不再检查boss血条，这个图环境干扰容易误判
				_, _, ok = GetNextArea(*sctx.Game, s.colorDetector)
				if ok {
					robotgo.MoveClick(635, 715)
					sleeper.Sleep(200)
					robotgo.MoveClick(935, 735)
				}
				return ok, nil
			}, false)
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Log(s.GetName(), s.GetMode(), "检测到下一步按钮,正在正常退出副本..."),
	}
}

func (s *StrategyImpl) handleScence4() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Wait(500),
		s.script.Log(s.GetName(), s.GetMode(), "正在前往第4个关卡"),
		s.script.Move([]string{"w", "d", "shift"}, 1_000),
		s.script.Move([]string{"a", "shift"}, 2_000),
		s.script.Move([]string{"s"}, 3_000),
		s.script.ChangeCameraAngleForX(x, y, -180, 3.24),
		s.script.Move([]string{"w", "shift"}, 7_500),
		s.script.ChangeCameraAngleForX(x, y, -67, 3.24),
		s.script.MoveAndOnce([]string{"w", "shift"}, 4_000, func(sc *strategy.StrategyContext) (bool, error) {
			robotgo.KeyTap("space")
			sleeper.Sleep(500)
			robotgo.KeyTap("space")
			sleeper.Sleep(500)
			robotgo.KeyTap("space")
			sleeper.Sleep(500)
			robotgo.KeyTap("space")
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Log(s.GetName(), s.GetMode(), "执行第4个关卡(特征:人型)"),
		s.script.TapOnce("h"),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			return utils.NewTicker(10*time.Minute, 300*time.Millisecond, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetBossConditionArea(*sc.Game, s.colorDetector)
				return ok, nil
			}, false)
		}, func() *strategy.StrategyContext { return s.context }),
	}
}

func (s *StrategyImpl) handleScence3() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Wait(500),
		s.script.Log(s.GetName(), s.GetMode(), "正在前往第3个关卡"),
		s.script.ChangeCameraAngleForX(x, y, -7, 3.24),
		s.script.Move([]string{"w", "shift"}, 15_000),
		s.script.Wait(5_000),
		s.script.ChangeCameraAngleForX(x, y, 4, 3.24),
		s.script.MoveAndOnce([]string{"w", "shift"}, 10_000, func(sc *strategy.StrategyContext) (bool, error) {
			sleeper.Sleep(6000)

			robotgo.KeyTap("e")
			sleeper.SleepBusyLoop(600)
			robotgo.KeyTap("e")
			sleeper.Sleep(1000)

			robotgo.KeyUp("shift")
			robotgo.KeyDown("shift")
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Log(s.GetName(), s.GetMode(), "执行第3个关卡(特征:人形)"),
		s.script.TapOnce("shift"),
		s.script.Wait(1_000),
		s.script.Move([]string{"d", "shift"}, 5_000),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			s.StopDeathCheck(sc)
			utils.NewTicker(50*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetRebirthLightArea(*sc.Game, s.colorDetector)
				if ok {
					robotgo.MoveClick(1123, 700)
					sleeper.Sleep(6_000)
				}
				return false, nil
			}, true)
			// 第4个关卡开始就不需要死亡检测了
			// s.StartDeathCheck(sc)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
	}
}

func (s *StrategyImpl) handleScence2() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Wait(700),
		s.script.Log(s.GetName(), s.GetMode(), "正在前往第2个关卡"),
		s.script.Move([]string{"w", "shift"}, 13_000),
		s.script.MoveAndOnce([]string{"a"}, 3_000, func(sc *strategy.StrategyContext) (bool, error) {
			robotgo.KeyTap("space")
			sleeper.Sleep(200)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Move([]string{"d"}, 400),
		s.script.ChangeCameraAngleForX(x, y, 55, 3.24),
		s.script.MoveAndOnce([]string{"w", "shift"}, 5_000, func(sc *strategy.StrategyContext) (bool, error) {
			sleeper.Sleep(1_000)
			robotgo.KeyTap("space")
			sleeper.Sleep(500)
			robotgo.KeyTap("space")
			sleeper.Sleep(500)
			robotgo.KeyTap("space")
			sleeper.Sleep(200)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Log(s.GetName(), s.GetMode(), "执行第2个关卡(特征:人型)"),
		s.script.Move([]string{"a"}, 1_500),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			s.StopDeathCheck(sc)
			utils.NewTicker(50*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetRebirthLightArea(*sc.Game, s.colorDetector)
				if ok {
					robotgo.MoveClick(1123, 700)
					sleeper.Sleep(6_000)
				}
				return false, nil
			}, true)
			s.StartDeathCheck(sc)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
	}
}

func (s *StrategyImpl) handleScence1() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Log(s.GetName(), s.GetMode(), "执行第1个关卡(特征:人型)"),
		s.script.Wait(2000),
		s.script.Move([]string{"w", "shift"}, 3_700),
		s.script.ChangeCameraAngleForX(x, y, -73, 3.24),
		s.script.Move([]string{"w", "shift"}, 1_000),
		s.script.Wait(400),
		s.script.Move([]string{"w", "shift"}, 7_000),
		s.script.Wait(10_000),
		s.script.Move([]string{"d", "shift"}, 2_000),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			s.StopDeathCheck(sc)
			utils.NewTicker(40*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetRebirthLightArea(*sc.Game, s.colorDetector)
				if ok {
					robotgo.MoveClick(1123, 700)
					sleeper.Sleep(6_000)
				}
				return false, nil
			}, true)
			s.StartDeathCheck(sc)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Move([]string{"s"}, 1_500),
		s.script.Move([]string{"w"}, 2_200),
		s.script.ChangeCameraAngleForX(x, y, 90, 3.24),
	}
}

func (s *StrategyImpl) startDungeon() []script.Operation {
	return []script.Operation{
		s.script.Wait(2000),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			ret, err := utils.NewTicker(20*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetDungeonExitArea(*sctx.Game, s.colorDetector) // todo: 这里有问题
				return ok, nil
			}, true)
			if !ret {
				log.Printf("[%s-%s] 检测到未进入地下城\n", s.GetName(), s.GetMode())
			} else {
				log.Printf("[%s-%s] 检测到已进入地下城\n", s.GetName(), s.GetMode())
			}
			return ret, err
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Wait(3000),
		s.script.Move([]string{"a"}, 400),
		s.script.Move([]string{"w"}, 900),
		s.script.TapOnce("f"),
		s.script.Log(s.GetName(), s.GetMode(), "正在开启地下城..."),
		s.script.Wait(200),
		s.script.MouseMoveClick(810, 665),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			ret, err := utils.NewTicker(15*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetDungeonRunningArea(*sctx.Game, s.colorDetector)
				return ok, nil
			}, true)
			if !ret {
				log.Printf("[%s-%s] 检测到未开启地下城\n", s.GetName(), s.GetMode())
			} else {
				log.Printf("[%s-%s] 检测到已开启地下城\n", s.GetName(), s.GetMode())
			}
			return ret, err
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.ExecTask(func(scxt *strategy.StrategyContext) (bool, error) {
			s.StartDeathCheck(scxt)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
	}
}

func (s *StrategyImpl) goToDungeon() []script.Operation {
	return []script.Operation{
		// 检查是否在地下城入口
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			return utils.NewTicker(10*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetMainArea(*sctx.Game, s.colorDetector)
				return ok, nil
			}, true)
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.TapOnce("f"),
		s.script.Wait(200),
		s.script.MouseMoveClick(124, 208),
		s.script.Wait(200),
		s.script.MouseMoveClick(1000, 690),
		s.script.Wait(200),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			if _, sizeList, ok := GetDungeonQueueArea(*sctx.Game, s.colorDetector); !ok || len(sizeList) != 2 {
				return false, nil
			}
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.MouseMoveClick(1185, 735),
		s.script.MouseMove(0, 0),
		s.script.Wait(3 * 1000),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			if _, sizeList, ok := GetDungeonQueueArea(*sctx.Game, s.colorDetector); ok && len(sizeList) == 2 {
				log.Printf("[%s-%s] 检测到异常队伍,正在执行退出队伍操作...\n", s.GetName(), s.GetMode())
				robotgo.KeyTap("esc")
				script.HandleAbnormalTeam(sctx.Game)
				s.run(s.goToDungeon())
			}
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Log(s.GetName(), s.GetMode(), "正在进入地下城..."),
	}
}
