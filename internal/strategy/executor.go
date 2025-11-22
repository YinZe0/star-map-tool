package strategy

import (
	"context"
	"errors"
	"log"
	"star-map-tool/internal/game"
	"star-map-tool/internal/listener"
	"time"
)

type Executor struct {
	selector *Selector
	result   ExecutionResult
}

type ExecutionConfig struct {
	Game     *game.Game
	Times    int                // 要执行的次数
	Timeout  time.Duration      // 每轮执行的超时时间，超时后放弃此轮执行，并开始下一轮
	Interval time.Duration      // 每轮执行的间隔
	Listener *listener.Listener // 执行状态控制器
}

type ExecutionResult struct {
	times   int
	success int
	fail    int
}

func NewExecutor(selector *Selector) *Executor {
	return &Executor{
		selector: selector,
	}
}

func (e *Executor) Execute(config *ExecutionConfig, strategy Strategy, data interface{}) {
	if config.Listener == nil {
		log.Println("[执行器] 未找到执行状态控制器,已退出程序!")
		return
	}
	strategy.Init()

	timeout := time.Duration(config.Timeout)
	for range config.Times {
		start := time.Now()
		log.Printf("[执行器] 开始执行第%d轮\n", e.result.times+1)

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		sctx := NewStrategyContext(config.Game)
		endReason := e.execute0(ctx, strategy, sctx, data)
		if endReason == STRATEGY_REASON_SUCCESS {
			e.record(true)
		} else {
			e.record(false)
		}

		elapsed := time.Since(start)
		log.Printf("[执行器] 本轮耗时%d秒", int(elapsed.Seconds()))
		log.Printf("[执行器] 已执行%d轮 成功%d轮 失败%d轮\n", e.result.times, e.result.success, e.result.fail)
		time.Sleep(config.Interval)
	}
}

func (e *Executor) execute0(ctx context.Context, strategy Strategy, sctx *StrategyContext, data any) int32 {
	done := make(chan bool, 1)
	defer close(done)
	start := time.Now()

	go func() {
		ok := strategy.Execute(sctx, data)
		game.ReleaseAllKey()
		done <- ok
	}()

	select {
	case <-ctx.Done():
		elapsed := time.Since(start)
		log.Printf("[执行器] 检测到本轮已执行%.2f分钟, 已达到超时条件, 即将进行P本并开始下一轮 \n", elapsed.Minutes())
		err := ctx.Err()

		var reason int32
		if errors.Is(err, context.DeadlineExceeded) {
			strategy.Abort(STRATEGY_EVENT_TIMEOUT) // 异步通知正在执行的线程应该停止了(停止是需要时间的)
			reason = STRATEGY_REASON_TIMEOUT
		} else {
			strategy.Abort(STRATEGY_EVENT_OTHER)
			reason = STRATEGY_REASON_OTHER
		}
		<-done // 等待子线程返回执行成功或者停止成功的信号
		return reason
	case result := <-done:
		if result {
			return STRATEGY_REASON_SUCCESS
		} else {
			return STRATEGY_REASON_FAIL
		}
	}
}

func (e *Executor) record(success bool) {
	if success {
		e.result.success = e.result.success + 1
	} else {
		e.result.fail = e.result.fail + 1
	}
	e.result.times = e.result.times + 1
}
