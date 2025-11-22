package listener

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-vgo/robotgo"
	hook "github.com/robotn/gohook"
)

// 负责监听用户输出的 F9/F10 等按键指令
type Listener struct {
	state int32
	Open  chan int
}

const (
	STATE_CREATE int32 = iota
	STATE_READY
	STATE_RUNNING
	STATE_STOPPING
)

// const STATE_SUPPEND int32 = 3 // 不考虑实现暂停

func New() *Listener {
	return &Listener{
		state: STATE_CREATE,
		Open:  make(chan int, 1),
	}
}

func (l *Listener) Start(ctx context.Context) {
	val := atomic.CompareAndSwapInt32(&l.state, STATE_CREATE, STATE_READY)
	if val {
		l.run0()
	}
}

func (l *Listener) run0() {
	hook.Register(hook.KeyDown, []string{"f9"}, func(e hook.Event) {
		if ok := atomic.CompareAndSwapInt32(&l.state, STATE_READY, STATE_RUNNING); ok {
			l.Open <- 1
			log.Println("[状态控制器] 检测到F9输入, 开始执行任务.")
		}
	})

	hook.Register(hook.KeyDown, []string{"f10"}, func(e hook.Event) {
		if ok := atomic.CompareAndSwapInt32(&l.state, STATE_RUNNING, STATE_STOPPING); ok {
			log.Println("[状态控制器] 检测到F10输入, 开始停止逻辑.")
			l.Release()
		}
	})

	fmt.Println("[状态控制器] 状态控制器已装载 F9:开始 F10:结束")
	fmt.Printf("\n")

	chain := hook.Start()
	defer func() {
		l.Release()
	}()

	go func() {
		<-hook.Process(chain) // 这东西会永久阻塞当前协程，造成defer无法执行，进而导致多次按键事件叠加
		log.Println("[状态控制器] 状态控制器已卸载")
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-signals
}

func (l *Listener) Release() {
	hook.End()
	time.Sleep(1 * time.Second) // 很奇怪的东西，hook关闭不彻底会导致下次启动失败(重启进程也不行)

	keysToReset := []string{"shift", "ctrl", "alt", "w", "a", "s", "d"}
	for _, key := range keysToReset {
		robotgo.KeyToggle(key, "up")
	}
	time.Sleep(200 * time.Millisecond)

	os.Exit(0) // 识别到F10，直接退出进程
}

func (l *Listener) IsRunning() bool {
	return atomic.LoadInt32(&l.state) == STATE_RUNNING
}
