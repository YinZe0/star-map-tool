package sheep2

import (
	_ "embed"
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
	dnnDetector   detector.DNNDetector
	script        script.Script
}

func NewStrategyImpl() strategy.Strategy {
	return &StrategyImpl{
		enable: 1,
	}
}

func (s *StrategyImpl) GetName() string {
	return "衰败深处"
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

//go:embed ..\..\..\..\assets\models\sbsc\best.onnx
var modeFile []byte

func (s *StrategyImpl) Init() {
	s.colorDetector = detector.NewColorDetector()
	s.dnnDetector = detector.NewDNNDetector("", modeFile)
	s.script = script.NewDefaultScript()
}

func (s *StrategyImpl) Execute(sctx *strategy.StrategyContext, data interface{}) bool {
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
		// 检测是否进入boss房间
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			return utils.NewTicker(20*time.Second, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetBossArea(*sctx.Game, s.colorDetector)
				return ok, nil
			}, true)
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Log(s.GetName(), s.GetMode(), "执行Boss关卡"),
		s.script.Wait(4_000),

		// 开怪
		s.script.Move([]string{"w", "shift"}, 300),
		s.script.TapOnce("h"), // 问题是Boss有秒杀技

		// 持续检查是否进入二阶段
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			utils.NewTicker(4*time.Minute, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetRebirthLightArea(*sctx.Game, s.colorDetector)
				if ok {
					robotgo.MoveClick(1123, 700)
				}
				_, _, ok = GetBossGrayHealth(*sctx.Game, s.colorDetector)
				return ok, nil
			}, true)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Wait(200),
		s.script.TapOnce("h"), // 过场动画中关H没用，得在外面关
		s.script.ExecTask(func(scxt *strategy.StrategyContext) (bool, error) {
			s.StartDeathCheck(scxt)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),

		// 通过转向找到目标钥匙
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			if ok := findAndFaceBoss(s, sctx, -45); !ok {
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
			ok := GotoTaskKey(s, sctx, 30_000)
			return ok, nil
		}, func() *strategy.StrategyContext { return s.context }),

		// 等待钥匙被AI击败后，找到并前往门的方向
		s.script.Wait(6_000),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			direction := sctx.Attrs["KeyDirection"].(int)
			angle := 45
			if direction == 1 {
				angle = -angle
			}

			if ok := findAndFaceBoss(s, sctx, angle); !ok {
				return false, errors.New("定位BOSS失败")
			}
			direction, ok := findDirectionOfWall(s, sctx)
			if !ok {
				return false, errors.New("定位墙体失败")
			}
			ok = GotoWall(s, sctx, direction, 40_000)
			return ok, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.ExecTask(func(scxt *strategy.StrategyContext) (bool, error) {
			s.StopDeathCheck(scxt)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.TapOnce("h"),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			for range 7 {
				script.ChangeCameraAngleForX(x, y, -50, 3.24)
			}
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),

		// 持续检查是否进入结算阶段
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			return utils.NewTicker(3*time.Minute, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetBossHealth(*sctx.Game, s.colorDetector)
				if !ok {
					log.Printf("[%s-%s] 检测到BOSS即将死亡...\n", s.GetName(), s.GetMode())
				}
				return !ok, nil
			}, false)
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Wait(10_000),
		s.script.Log(s.GetName(), s.GetMode(), "开始检测结算画面..."),
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			return utils.NewTicker(12*time.Minute, 1*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("策略已停止")
				}
				_, _, ok := GetNextArea(*sctx.Game, s.colorDetector)
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

func (s *StrategyImpl) handleScence5() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Log(s.GetName(), s.GetMode(), "执行第5个关卡(特征:胖子)"),

		s.script.Wait(4_000), // 回体力用
		s.script.Move([]string{"s"}, 500),
		s.script.ChangeCameraAngleForX(x, y, 92, 3.24),
		s.script.Move([]string{"w", "shift"}, 5_000),
		s.script.Wait(1000),
		s.script.MouseClick(),
		s.script.Wait(400),
		s.script.MoveAndKeep([]string{"d", "shift"}, 50_000, 1000, func(sctx *strategy.StrategyContext) (bool, error) {
			if !s.IsEnable() {
				return false, errors.New("策略已停止")
			}
			_, _, ok := GetBossConditionArea(*sctx.Game, s.colorDetector)
			return ok, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.Move([]string{"d", "shift"}, 5_000), // 这里会被吸走，全凭移动 + ai奶尽可能幸存

		s.script.ChangeCameraAngleForX(x, y, -38, 3.24),
		s.script.Move([]string{"w", "shift"}, 7_000),
		s.script.ChangeCameraAngleForX(x, y, 33, 3.24),
		s.script.Move([]string{"d"}, 800),

		// 在走过场之前关闭死亡检测，不然死亡检测会因为过场动画误判
		s.script.ExecTask(func(sctx *strategy.StrategyContext) (bool, error) {
			s.StopDeathCheck(sctx)
			return true, nil
		}, func() *strategy.StrategyContext { return s.context }),
		s.script.MoveAndKeep([]string{"w", "shift"}, 10_000, 1_000, func(sctx *strategy.StrategyContext) (bool, error) {
			if !s.IsEnable() {
				return false, errors.New("策略已停止")
			}
			_, _, ok := GetPlayerHealthArea(*sctx.Game, s.colorDetector)
			return !ok, nil
		}, func() *strategy.StrategyContext { return s.context }),
	}
}

func (s *StrategyImpl) handleScence4() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Log(s.GetName(), s.GetMode(), "执行第4个关卡(特征:羊)"),

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

func (s *StrategyImpl) handleScence3() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Log(s.GetName(), s.GetMode(), "执行第3个关卡(特征:姆克)"),

		s.script.Move([]string{"w", "shift"}, 5000),

		s.script.ChangeCameraAngleForX(x, y, -52, 3.24),
		s.script.Move([]string{"w", "shift"}, 9000),

		s.script.ChangeCameraAngleForX(x, y, 180, 3.24),
		s.script.Wait(30_000),
		s.script.ChangeCameraAngleForX(x, y, -180, 3.24),
	}
}

func (s *StrategyImpl) handleScence2() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Log(s.GetName(), s.GetMode(), "执行第2个关卡(特征:羊)"),

		s.script.Wait(500),
		s.script.Move([]string{"d", "shift"}, 1000),
		s.script.Move([]string{"a"}, 800),
		s.script.Move([]string{"w", "shift"}, 500),
		s.script.Move([]string{"a", "shift"}, 4000),
		// s.script.Wait(30_000),
		s.script.Wait(15_000),

		s.script.ChangeCameraAngleForX(x, y, 17, 3.24),
		s.script.Move([]string{"w", "shift"}, 7000),
		s.script.Wait(2000), // 等待加速BUFF消失
		s.script.Move([]string{"s"}, 400),

		s.script.ChangeCameraAngleForX(x, y, 85, 3.24),
		s.script.Move([]string{"w", "shift"}, 3500),
		s.script.Move([]string{"d", "shift"}, 1000),
		s.script.ChangeCameraAngleForX(x, y, -17, 3.24),
	}
}

func (s *StrategyImpl) handleScence1() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Log(s.GetName(), s.GetMode(), "执行第1个关卡(特征:蜥蜴)"),
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
			if !s.IsEnable() {
				return false, errors.New("策略已停止")
			}
			startTime := sctx.Attrs["_scence1_start_time"].(time.Time)
			d := sctx.Attrs["_scence1_direction"].(int32)
			if d == 0 {
				d = 1
				robotgo.KeyDown("ctrl")
				robotgo.KeyDown("w")
			}

			_, _, ok := GetSwordKey1Area(*sctx.Game, s.colorDetector)
			if time.Since(startTime) < 12*time.Second { // 12秒的机会去不停的找剑，找不到就继续往后走，找到就恢复往前走
				if !ok {
					robotgo.KeyUp("w")
					robotgo.KeyUp("ctrl")
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
