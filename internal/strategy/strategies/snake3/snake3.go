package snake3

import (
	"errors"
	"log"
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
	return "岩蛇巢穴"
}

func (s *StrategyImpl) GetMode() string {
	return "大师1"
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

	// 目前无法识别是否到达指定地点，全是按照时间进行的，后续可以追加训练一些标志物识别的模型
	var operationList []script.Operation
	operationList = append(operationList, s.goToDungeon()...)
	operationList = append(operationList, s.startDungeon()...)
	operationList = append(operationList, s.handleScence1()...)
	operationList = append(operationList, s.handleScence2()...)
	operationList = append(operationList, s.handleScence3()...)
	operationList = append(operationList, s.handleScence4()...)
	operationList = append(operationList, s.handleScence5()...)
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
		s.script.Wait(6_000),
		s.script.Log(s.GetName(), s.GetMode(), "执行第Boss关卡"),
		s.script.TapOnce("h"),

		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			return utils.NewTicker(15*time.Minute, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}

				// 不再检查boss血条，这个图环境干扰容易误判
				if _, _, ok := GetNextArea(*sctx.Game, s.colorDetector); ok {
					sleeper.SleepBusyLoop(1_500) // 转视角会占用鼠标事件，等待一会儿再点击
					robotgo.MoveClick(635, 715)
					sleeper.SleepBusyLoop(200)
					robotgo.MoveClick(935, 735)
					return true, nil
				}

				if _, _, ok := GetRebirthLightArea(*sctx.Game, s.colorDetector); ok {
					robotgo.MoveClick(1123, 700)
				} else {
					script.ChangeCameraAngleForX(x, y, -60, 3.24)
				}
				return false, nil
			}, false)
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Log(s.GetName(), s.GetMode(), "检测到下一步按钮,正在正常退出副本..."),
	}
}

func (s *StrategyImpl) handleScence5() []script.Operation {
	return []script.Operation{
		s.script.Wait(1_000),
		s.script.Log(s.GetName(), s.GetMode(), "执行第5个关卡(特征:矿车)"),
		s.script.TapOnce("f"),

		s.script.Wait(50_000),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			return utils.NewTicker(20*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetPlayerHealthArea(*sc.Game, s.colorDetector)
				return !ok, nil
			}, true)
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Log(s.GetName(), s.GetMode(), "矿车环节结束,正在进入Boss战..."),
	}
}

func (s *StrategyImpl) handleScence4() []script.Operation {
	return []script.Operation{
		s.script.Log(s.GetName(), s.GetMode(), "执行第4个关卡(特征:蜘蛛)"),
		s.script.Move([]string{"a", "shift"}, 8_500),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			s.StopDeathCheck(sc)
			s.script.Move([]string{"w"}, 4_000)()
			return utils.NewTicker(80*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetBossConditionArea(*sc.Game, s.colorDetector)
				return ok, nil
			}, true)
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Move([]string{"a"}, 2_000),
		s.script.Move([]string{"w", "shift"}, 3_000),
	}
}

func (s *StrategyImpl) handleScence3() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.MouseClick(),
		s.script.Wait(600),
		s.script.Log(s.GetName(), s.GetMode(), "执行第3个关卡(特征:蜘蛛)"),
		s.script.ChangeCameraAngleForX(x, y, -87, 3.24),
		s.script.Move([]string{"w", "shift"}, 3_000),
		s.script.ChangeCameraAngleForX(x, y, 30, 3.24),
		s.script.Move([]string{"s", "shift"}, 6_000), // 4_000

		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			s.StopDeathCheck(sc)
			utils.NewTicker(40*time.Second, 1*time.Second, func() (bool, error) { // 这里等暴怒，因为暴怒会影响最终走向矿车的判断
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetRebirthLightArea(*sc.Game, s.colorDetector)
				if ok {
					robotgo.MoveClick(1123, 700)
				}
				return false, nil
			}, true)
			s.StartDeathCheck(sc)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Move([]string{"s"}, 2_000),
		s.script.ChangeCameraAngleForX(x, y, -24, 3.24),

		// s.script.Move([]string{"w", "shift"}, 4000),
		// s.script.TapOnce("e"),
		// s.script.Wait(600),
		// s.script.TapOnce("e"),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			robotgo.KeyDown("w")
			robotgo.KeyDown("shift")

			sleeper.SleepBusyLoop(4_000)
			utils.NewTicker(5*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				robotgo.KeyTap("space")
				return false, nil
			}, true)
			robotgo.KeyUp("shift")
			robotgo.KeyUp("w")
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),

		s.script.Move([]string{"s"}, 200),
		s.script.Move([]string{"d"}, 3_000),
		s.script.Wait(4000), // 等待加速消失
	}
}

func (s *StrategyImpl) handleScence2() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Wait(1500),
		s.script.Log(s.GetName(), s.GetMode(), "执行第2个关卡(特征:蜥蜴)"),
		s.script.ChangeCameraAngleForX(x, y, 90, 3.24),
		s.script.Wait(600),

		s.script.Move([]string{"w", "shift"}, 7600),
		s.script.ChangeCameraAngleForX(x, y, -135, 3.24),

		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			s.StopDeathCheck(sc)
			utils.NewTicker(45*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetRebirthLightArea(*sc.Game, s.colorDetector)
				if ok {
					robotgo.MoveClick(1123, 700)
				}
				return false, nil
			}, true)
			s.StartDeathCheck(sc)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),

		s.script.MoveAndOnce([]string{"w", "shift"}, 8_000, func(sctx *strategy.StrategyContext) (bool, error) {
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
}

func (s *StrategyImpl) handleScence1() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Log(s.GetName(), s.GetMode(), "执行第1个关卡(特征:蜘蛛)"),
		s.script.Wait(2000),

		s.script.Move([]string{"w", "shift"}, 4000),
		// s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
		// 	s.StopDeathCheck(sc)
		// 	utils.NewTicker(40*time.Second, 1*time.Second, func() (bool, error) {
		// 		if !s.IsEnable() {
		// 			return false, errors.New("策略已停止")
		// 		}
		// 		_, _, ok := GetRebirthLightArea(*sc.Game, s.colorDetector)
		// 		if ok {
		// 			robotgo.MoveClick(1123, 700)
		// 		}
		// 		return false, nil
		// 	}, true)
		// 	s.StartDeathCheck(sc)
		// 	return true, nil
		// }, func() *strategy.StrategyContext { return s.context }),
		s.script.Move([]string{"s"}, 1000),
		s.script.ChangeCameraAngleForX(x, y, -70, 3.24),

		s.script.MoveAndOnce([]string{"w", "shift"}, 3500, func(sctx *strategy.StrategyContext) (bool, error) {
			robotgo.KeyTap("e")
			sleeper.SleepBusyLoop(600)
			robotgo.KeyTap("e")
			sleeper.Sleep(200)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.MouseMove(x, y),
		s.script.ChangeCameraAngleForX(x, y, -78, 3.24),
		s.script.Wait(600),

		s.script.Move([]string{"w", "shift"}, 13_000),
		s.script.ChangeCameraAngleForX(x, y, 95, 3.24),

		s.script.Move([]string{"w", "shift"}, 5000),
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
		s.script.Move([]string{"w"}, 1200),
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
		s.script.MouseMoveClick(121, 272), // 选择大师难度
		s.script.Wait(200),
		// 拖拽后选择大师1
		s.script.MouseMove(514, 732),
		s.script.Wait(200),
		s.script.MouseDragSmooth(946, 726, 2.0),
		s.script.Wait(200),
		s.script.MouseMoveClick(464, 735),
		// 进入副本
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
