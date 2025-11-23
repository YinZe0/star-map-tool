package script

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"star-map-tool/internal/game"
	"star-map-tool/internal/pkg/sleeper"
	"star-map-tool/internal/pkg/utils"
	"star-map-tool/internal/strategy"

	"github.com/go-vgo/robotgo"
)

// 向前滑翔 w(down) + 2x space(tap) + q(tap)
// 向前滑翔 w(down) + 2x space(tap) + q(tap) + left(tap)
// 爬墙     w(down)
// 边走边跳 w(down) + 2x space(tap)
// 奔跑     w(down) + shift(tap)
// 慢走     w(down) + ctrl(down)

// 要求接口传入的时限时长都是毫秒数
type Script interface {
	Move(keys []string, duration int) Operation
	MoveAndOnce(keys []string, duration int, task func(*strategy.StrategyContext) (bool, error), getsctx func() *strategy.StrategyContext) Operation
	MoveAndKeep(keys []string, duration int, interval int,
		task func(*strategy.StrategyContext) (bool, error), getsctx func() *strategy.StrategyContext) Operation

	TapOnce(key string) Operation
	Tap(key string, times int, interval int) Operation

	Scroll(x int, direction string) Operation
	MouseClick() Operation
	MouseMove(x, Y int) Operation
	MouseMoveClick(x, y int) Operation
	ChangeCameraAngleForX(x int, y int, angle int, baseline float32) Operation
	ChangeCameraAngleForY(x int, y int, angle int, baseline float32) Operation

	Wait(duration int) Operation
	ExecTask(task func(*strategy.StrategyContext) (bool, error), getsctx func() *strategy.StrategyContext) Operation
	Log(name string, mode string, message string) Operation
}

type Operation func() bool

type DefaultScript struct{}

func NewDefaultScript() Script {
	return &DefaultScript{}
}

func (s *DefaultScript) Move(keys []string, duration int) Operation {
	return func() bool {
		for i := range keys {
			robotgo.KeyDown(keys[i])
		}

		sleeper.SleepBusyLoop(duration)

		for i := len(keys) - 1; i >= 0; i-- {
			robotgo.KeyUp(keys[i])
		}
		return true
	}
}

func (s *DefaultScript) MoveAndOnce(keys []string, duration int,
	task func(*strategy.StrategyContext) (bool, error), getsctx func() *strategy.StrategyContext) Operation {
	return func() bool {
		sctx := getsctx()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for i := range keys {
			robotgo.KeyDown(keys[i])
		}

		// once 逻辑 (给子逻辑一次执行机会)
		go func() {
			once(ctx, task, sctx)
		}()
		sleeper.SleepBusyLoop(duration)
		cancel()

		for i := len(keys) - 1; i >= 0; i-- {
			robotgo.KeyUp(keys[i])
		}
		return true
	}
}

func (s *DefaultScript) MoveAndKeep(keys []string, duration int, interval int,
	task func(*strategy.StrategyContext) (bool, error), getsctx func() *strategy.StrategyContext) Operation {
	return func() bool {
		sctx := getsctx()

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(duration)*time.Millisecond)
		defer cancel()

		done := make(chan int, 1) // (1成功，2超时, 3失败)
		defer close(done)

		for i := range keys {
			robotgo.KeyDown(keys[i])
		}

		// keep逻辑 (不停的执行子逻辑)
		go func() {
			keep(ctx, task, sctx, interval, done)
		}()
		result := <-done // 放行条件：超时 或者 子任务结束后主动关闭

		for i := len(keys) - 1; i >= 0; i-- {
			robotgo.KeyUp(keys[i])
		}
		return result == 1
	}
}

func (s *DefaultScript) TapOnce(key string) Operation {
	return func() bool {
		robotgo.KeyTap(key)
		time.Sleep(200 * time.Millisecond)

		return true
	}
}

func (s *DefaultScript) Tap(key string, times int, interval int) Operation {
	return func() bool {
		for i := range times {
			robotgo.KeyTap(key)

			if i+1 != times && interval > 0 {
				time.Sleep(time.Duration(interval) * time.Millisecond)
			}
		}
		time.Sleep(200 * time.Millisecond)
		return true
	}
}

func (s *DefaultScript) Scroll(x int, direction string) Operation {
	return func() bool {
		robotgo.ScrollDir(x, direction)
		return true
	}
}

func (s *DefaultScript) MouseClick() Operation {
	return func() bool {
		robotgo.Click()
		return true
	}
}

func (s *DefaultScript) MouseMove(x, Y int) Operation {
	return func() bool {
		robotgo.Move(x, Y)
		return true
	}
}

func (s *DefaultScript) MouseMoveClick(x, y int) Operation {
	return func() bool {
		robotgo.MoveClick(x, y)
		return true
	}
}

func (s *DefaultScript) ChangeCameraAngleForX(x int, y int, angle int, baseline float32) Operation {
	// 这里求出来的并不是准确的，需要根据场景做调整(当镜头与角色距离不同时，得到的结果是不同的)
	// 当默认镜头距离时(滚轮5个单位)，每3.24像素近似于1度；当最小镜头距离时，每3.24像素近似于1度
	return func() bool {
		ChangeCameraAngleForX(x, y, angle, baseline)
		return true
	}
}

func (s *DefaultScript) ChangeCameraAngleForY(x int, y int, angle int, baseline float32) Operation {
	// 7.8 随便选的
	return func() bool {
		ChangeCameraAngleForY(x, y, angle, baseline)
		return true
	}
}

func (s *DefaultScript) Wait(duration int) Operation {
	return func() bool {
		sleeper.SleepBusyLoop(duration)
		return true
	}
}

func (s *DefaultScript) ExecTask(task func(*strategy.StrategyContext) (bool, error), getsctx func() *strategy.StrategyContext) Operation {
	return func() bool {
		sctx := getsctx()

		ok, err := task(sctx)
		if err != nil {
			return false
		}
		return ok
	}
}

func (s *DefaultScript) Log(name string, mode string, message string) Operation {
	return func() bool {
		log.Printf("[%s-%s] %s\n", name, mode, message)
		return true
	}
}

// -------------------------------------- 额外方法 ------------------------------------------------

func once(ctx context.Context, task func(*strategy.StrategyContext) (bool, error), sctx *strategy.StrategyContext) {
	interval := time.Duration(1) * time.Second
	timer := time.NewTimer(interval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
	case <-timer.C:
		task(sctx)
	}
}

func keep(ctx context.Context, task func(*strategy.StrategyContext) (bool, error), sctx *strategy.StrategyContext, intervalMs int, done chan int) {
	interval := time.Duration(intervalMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			done <- 2 // 执行超时
			return
		case <-ticker.C:
			ok, err := task(sctx)
			if ok {
				done <- 1 // 执行完成
				return
			}
			if err != nil {
				done <- 3 // 执行异常
				return
			}
		}
	}
}

func ChangeCameraAngleForX(x int, y int, angle int, baseline float32) {
	// 这里求出来的并不是准确的，需要根据场景做调整(当镜头与角色距离不同时，得到的结果是不同的)
	// 当默认镜头距离时(滚轮5个单位)，每3.24像素近似于1度；当最小镜头距离时，每3.24像素近似于1度
	robotgo.KeyDown("alt")
	robotgo.Toggle("left")

	time.Sleep(time.Duration(50) * time.Millisecond) // 等待上方事件起作用 （键盘和鼠标衔接的地方仍要等待）
	offsetX := int(float32(angle) * baseline)
	robotgo.Move(x+offsetX, y)

	robotgo.Toggle("left", "up")
	robotgo.KeyUp("alt")
}

func ChangeCameraAngleForY(x int, y int, angle int, baseline float32) {
	robotgo.KeyDown("alt")
	robotgo.Toggle("left")

	time.Sleep(time.Duration(50) * time.Millisecond) // 等待上方事件起作用 （键盘和鼠标衔接的地方仍要等待）
	offsetY := int(float32(angle) * baseline)
	robotgo.Move(x, y+offsetY)

	robotgo.Toggle("left", "up")
	robotgo.KeyUp("alt")
}

func Scroll(x int, direction string) {
	robotgo.ScrollDir(x, direction)
}

func MoveCircle(duration int, interval int, signal *int32) bool {
	var state int32 = 1

	start := time.Now()
	ok, _ := utils.NewTicker(time.Duration(duration)*time.Millisecond, time.Duration(interval)*time.Millisecond, func() (bool, error) {
		if val := atomic.LoadInt32(signal); val == 1 {
			return true, nil // 收到通知就停下
		}

		s := atomic.LoadInt32(&state)
		switch s {
		case 1:
			robotgo.KeyUp("a")
			robotgo.KeyDown("s") // s
			atomic.StoreInt32(&state, 2)
		case 2:
			robotgo.KeyDown("d") // s + d
			atomic.StoreInt32(&state, 3)
		case 3:
			robotgo.KeyUp("s") // d
			atomic.StoreInt32(&state, 4)
		case 4:
			robotgo.KeyDown("w") // d + w
			atomic.StoreInt32(&state, 5)
		case 5:
			robotgo.KeyUp("d") // w
			atomic.StoreInt32(&state, 6)
		case 6:
			robotgo.KeyDown("a") // w + a
			atomic.StoreInt32(&state, 7)
		case 7:
			robotgo.KeyUp("w") // a
			atomic.StoreInt32(&state, 8)
		case 8:
			robotgo.KeyDown("s") // a + s
			atomic.StoreInt32(&state, 1)
		}

		if time.Since(start) > time.Duration(duration)*time.Millisecond {
			return false, errors.New("超时")
		}
		return false, nil
	}, true)
	return ok
}

func MoveSide(direction int, interval int, speed int, stop chan int) bool {
	var moveKeyList = []string{"w", "a", "s", "d", "shift", "ctrl"}
	var speedList = []string{"ctrl", "", "shift"}
	times := 0

	var stepList []string
	switch direction {
	case -1:
		// stepList = []string{"a,w", "w,a", "w"}
		stepList = []string{"a,w", "w", "w"}
	case 1:
		// stepList = []string{"d,w", "w,d", "w"} // 游戏有设计问题，光剑回到墙外边，即使人经过指定区域也不会判定生效
		stepList = []string{"d,w", "w", "w"}
	}

	x, y := robotgo.Location()
	getCurrStepIndex := func(t int) int {
		if t >= 3 {
			return t % 3
		} else {
			return t
		}
	}
	getPrevStepIndex := func(t int) int {
		if t == 0 {
			return 0
		} else if t <= 2 {
			return t - 1
		}

		r := t % 3
		if r == 0 {
			return 2
		} else {
			r = r - 1
		}
		return r
	}
	move := func(stepList []string, times int) {
		speedKey := speedList[speed+1]
		if len(stepList) > 0 {
			robotgo.KeyUp(speedKey)
		}
		prevKeys := stepList[getPrevStepIndex(times)]
		list := strings.SplitSeq(prevKeys, ",")
		for k := range list {
			robotgo.KeyUp(k)
		}
		currKeys := stepList[getCurrStepIndex(times)]
		list = strings.SplitSeq(currKeys, ",")
		for k := range list {
			robotgo.KeyDown(k)
		}
		if len(stepList) > 0 {
			robotgo.KeyDown(speedKey)
		}
	}
	move(stepList, times)
	times++

	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			for _, k := range moveKeyList {
				robotgo.KeyUp(k)
			}
			return true
		case <-ticker.C:
			if times > 0 {
				ChangeCameraAngleForX(x, y, -direction*24, 3.24)
			}
			move(stepList, times)
			times++
		}
	}
}

func HandleAbnormalTeam(game *game.Game) {
	robotgo.KeyTap("i")
	time.Sleep(time.Duration(3) * time.Second)

	// 点击退出队伍
	robotgo.MoveClick(1167, 730)

	// 确认退出队伍
	robotgo.MoveClick(795, 580)

	robotgo.MoveClick(1233, 66)
}
