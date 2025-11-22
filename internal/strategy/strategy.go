package strategy

import (
	"star-map-tool/internal/game"
	"sync"
)

type Strategy interface {
	GetName() string
	GetMode() string
	Init()
	Execute(sctx *StrategyContext, data interface{}) bool
	Abort(sign string)
}

type StrategyContext struct {
	Game   *game.Game
	Attrs  map[string]any // 不限制存储内容，如果是大型value，自动写入指针
	RWLock sync.RWMutex   // 提供对于 Attrs 的读写锁，可供其安全读写

	DeathCheckFlag int32 // 0关闭、1打开
	Step           int32 // 策略执行进度
}

// 成功、超时、终止、其它
const (
	STRATEGY_REASON_SUCCESS int32 = iota
	STRATEGY_REASON_FAIL
	STRATEGY_REASON_TIMEOUT
	STRATEGY_REASON_ABORT
	STRATEGY_REASON_OTHER
)

// 执行超时、用户取消
const (
	STRATEGY_EVENT_TIMEOUT string = "timeout"
	STRATEGY_EVENT_OTHER   string = "other"
)

func NewStrategyContext(game *game.Game) *StrategyContext {
	attrs := make(map[string]interface{})

	return &StrategyContext{Game: game, Attrs: attrs}
}
