package farm60

import (
	"errors"
	"star-map-tool/internal/detector"
	"star-map-tool/internal/pkg/script"
	"star-map-tool/internal/pkg/sleeper"
	"star-map-tool/internal/pkg/utils"
	"star-map-tool/internal/strategy"
	"star-map-tool/internal/strategy/preset"
	"sync/atomic"
	"time"

	"github.com/go-vgo/robotgo"
)

type Farm60Strategy struct {
	enable  int32
	context *strategy.StrategyContext

	colorDetector detector.ColorDetector
	script        script.Script
}

func NewFarm60Strategy() strategy.Strategy {
	return &Farm60Strategy{
		enable: 1,
	}
}

func (s *Farm60Strategy) GetName() string {
	return "虫茧"
}

func (s *Farm60Strategy) GetMode() string {
	return "60"
}

func (s *Farm60Strategy) IsEnable() bool {
	return atomic.LoadInt32(&s.enable) == 1
}

func (s *Farm60Strategy) Enable() {
	atomic.StoreInt32(&s.enable, 1)
}

func (s *Farm60Strategy) Disable(reason int32) {
	// -1: 终止(超时)、-2: 执行失败、-3: 死亡、0: 执行结束、1: 可用
	atomic.StoreInt32(&s.enable, reason)
}

func (s *Farm60Strategy) StartDeathCheck(ctx *strategy.StrategyContext) {
	atomic.StoreInt32(&ctx.DeathCheckFlag, 1) // 开启死亡检测
}

func (s *Farm60Strategy) StopDeathCheck(ctx *strategy.StrategyContext) {
	atomic.StoreInt32(&ctx.DeathCheckFlag, 0) // 关闭死亡检测
}

func (s *Farm60Strategy) Init() {
	s.colorDetector = detector.NewColorDetector()
	s.script = script.NewDefaultScript()
}

func (s *Farm60Strategy) Execute(sctx *strategy.StrategyContext, data interface{}) bool {
	s.context = sctx // 每次的策略上下文都是新的
	s.context.Attrs["START_TIME"] = time.Now()
	s.Enable()

	// 目前无法识别是否到达指定地点，全是按照时间进行的，后续可以追加训练一些标志物识别的模型
	var operationList []script.Operation
	operationList = append(operationList, s.execute0()...)
	return s.run(operationList)
}

func (s *Farm60Strategy) Abort(sign string) {
	s.Disable(-1)
}

func (s *Farm60Strategy) run(list []script.Operation) bool {
	for _, op := range list {
		if !s.IsEnable() {
			return false
		}
		ok := op()
		if !ok {
			s.Disable(-2)
			return false
		}
	}
	s.Disable(0) // 让子线程有停止的机会
	return true
}

func (s *Farm60Strategy) execute0() []script.Operation {
	x, y := robotgo.Location()
	return []script.Operation{
		s.script.Wait(1000),
		s.script.TapOnce("h"), // 关闭H
		s.script.Wait(10_000),
		s.script.TapOnce("m"),
		s.script.Wait(1000),
		s.script.MouseMoveClick(511, 380),
		s.script.Wait(1000),
		s.script.MouseMoveClick(1072, 731),
		s.script.MouseMoveClick(793, 578),
		s.script.Wait(10_000),
		s.script.TapOnce("m"),
		s.script.Wait(1_000),
		s.script.MouseMoveClick(474, 390),
		s.script.Wait(1_000),
		s.script.MouseMoveClick(979, 725),
		s.script.Wait(12_000),
		s.script.TapOnce("f"),
		s.script.Wait(300),
		s.script.TapOnce("g"),
		s.script.Wait(300),
		s.script.TapOnce("h"),
		s.script.Wait(300),
		s.script.ChangeCameraAngleForY(x, y, 90, 7.8),
		s.script.ExecTask(func(sc *strategy.StrategyContext) (bool, error) {
			// 如果超时标志没了，就重新进入副本
			return utils.NewTicker(1*time.Hour, 10*time.Second, func() (bool, error) {
				if !s.IsEnable() {
					return false, errors.New("无执行许可")
				}

				now := time.Now()
				if now.Hour() == 5 && now.Minute() == 0 && now.Second() < 30 { // 凌晨5点看不到血条就按下esc
					_, _, ok := preset.GetPlayerHealthArea(*sc.Game, s.colorDetector)
					if !ok {
						sleeper.Sleep(500)
						robotgo.MoveClick(640, 120)
						sleeper.Sleep(1_000)
						robotgo.MoveClick(1051, 214)
						sleeper.Sleep(1_000)
						robotgo.MoveClick(640, 120)
						sleeper.Sleep(2_000)
					}
				}
				_, _, ok := GetFarmStateArea(*sc.Game, s.colorDetector)
				return !ok, nil
			}, false)
		}, func() *strategy.StrategyContext { return s.context }),
	}
}
