package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"star-map-tool/internal/game"
	"star-map-tool/internal/listener"
	"star-map-tool/internal/strategy"
	"star-map-tool/internal/strategy/strategies/sbsc2"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/tailscale/win"
)

type Config struct {
	Map      string
	Mode     string
	Times    int
	Timeout  int
	Interval int
}

func RegisterStrategies(registry *strategy.Registry) {
	registry.Register(sbsc2.NewSbsc2Strategy())
}

func resizeCli() {
	// title := robotgo.GetTitle()
	title := "管理员: Windows PowerShell"

	hwnd := robotgo.FindWindow(title)
	val := win.SetWindowPos(hwnd, win.HWND_TOP, 1280, 0, 1920-1280, 800, win.SWP_SHOWWINDOW)
	if !val {
		panic("调整窗口大小失败")
	}
}

func main() {
	config := parseFlags()
	if err := validateConfig(config); err != nil {
		log.Printf("[启动器] 配置错误: %v\n", err)
		return
	}
	log.Printf("[启动器] 参数识别 [地图:%s] [模式:%s] [次数:%d] [单轮限时分钟数:%d] [下一轮开始前等待秒数:%d]\n",
		config.Map, config.Mode, config.Times, config.Timeout, config.Interval)
	resizeCli()

	// 游戏窗体 或 进程
	game, err := game.NewGame("Star.exe", "星痕共鸣")
	if err != nil {
		log.Println("[启动器] ", err)
		return
	}
	if ok := game.Initialize(); !ok {
		return
	}

	// 游戏策略选择
	registry := strategy.NewRegistry()
	RegisterStrategies(registry)

	selector := strategy.NewSelector(registry)
	executor := strategy.NewExecutor(selector)

	// 特殊按键监听器
	ctx, _ := context.WithCancel(context.Background())
	listener := listener.New()
	go listener.Start(ctx)

	for {
		// 等待F9
		<-listener.Open
		game.Active()

		// 执行游戏策略
		data := map[string]string{}
		executor.Execute(&strategy.ExecutionConfig{
			Game:     game,
			Times:    config.Times,
			Timeout:  time.Duration(config.Timeout) * time.Minute,
			Interval: time.Duration(config.Interval) * time.Second,
			Listener: listener,
		}, *selector.Select(config.Map, config.Mode), data)
	}
}

func parseFlags() Config {
	var options Config

	flag.StringVar(&options.Map, "map", "衰败深处", "地图 (默认: 衰败深处) ")
	flag.StringVar(&options.Mode, "mode", "困难", "模式 (默认: 普通)")
	flag.IntVar(&options.Times, "times", 999, "进行次数 (默认: 999)")
	flag.IntVar(&options.Timeout, "timeout", 10, "单轮限时分钟数 (默认: 11分钟) 超时后将自动P本进行下一轮")
	flag.IntVar(&options.Interval, "interval", 15, "每轮间隔秒数 (默认: 15秒)")

	flag.Parse()
	return options
}

func validateConfig(config Config) error {
	if config.Times < 1 || config.Times > 999 {
		return fmt.Errorf("进行次数必须在 1 到 999 之间")
	}
	if config.Timeout < 1 || config.Timeout > 20 {
		return fmt.Errorf("单轮限时分钟数必须在 1 到 20 之间")
	}
	if config.Interval < 10 || config.Interval > 60 {

	}
	return nil
}
