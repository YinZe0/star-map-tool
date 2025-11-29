package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"star-map-tool/internal/game"
	"star-map-tool/internal/listener"
	"star-map-tool/internal/strategy"
	"star-map-tool/internal/strategy/strategies/farm60"
	"star-map-tool/internal/strategy/strategies/hljs2"
	"star-map-tool/internal/strategy/strategies/sbsc2"
	"star-map-tool/internal/strategy/strategies/yscx2"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-vgo/robotgo"
	"github.com/tailscale/win"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procSetConsoleTitleW = kernel32.NewProc("SetConsoleTitleW")
)

type Config struct {
	Map         string
	Mode        string
	Times       int
	Timeout     int
	Interval    int
	Description string
}

const Title string = "星痕共鸣-S2刷图工具"

var Options []Config = []Config{
	{Map: "衰败深处", Mode: "困难", Times: 999, Timeout: 10, Interval: 10, Description: "请让出奶位, 部分环节存在奶量压力!"},
	{Map: "岩蛇巢穴", Mode: "困难", Times: 999, Timeout: 12, Interval: 10, Description: "请让出奶位, 部分环节存在奶量压力!"},
	{Map: "荒灵祭所", Mode: "困难", Times: 999, Timeout: 9, Interval: 10, Description: "请让出奶位, 部分环节存在奶量压力!"},
	// {Map: "无音之都", Mode: "困难", Times: 999, Timeout: 12, Interval: 10, Description: "请让出奶位, 部分环节存在奶量压力!"}, // 副本有BUG，吃不到球造成团灭
	{Map: "虫茧", Mode: "60", Times: 999, Timeout: 0, Interval: 1, Description: "需自行前往60虫茧外, 开启H后再启用副本方案"},
}

func RegisterStrategies(registry *strategy.Registry) {
	registry.Register(sbsc2.NewSbsc2Strategy())
	registry.Register(yscx2.NewYscx2Strategy())
	registry.Register(hljs2.NewHljs2Strategy())
	// registry.Register(wyzd2.NewWyzd2Strategy())
	registry.Register(farm60.NewFarm60Strategy())
}

func main() {
	defer handlePanic()

	SetConsoleTitle(Title)
	showReadMe()
	config := parseScan()

	fmt.Printf("[启动器] 参数识别 [地图:%s] [模式:%s] [次数:%d] [单轮限时分钟数:%d] [下一轮开始前等待秒数:%d]\n",
		config.Map, config.Mode, config.Times, config.Timeout, config.Interval)
	resizeCli()
	showMapDescripion(config)

	// 游戏窗体 或 进程
	game, err := game.NewGame("Star.exe", "星痕共鸣")
	if err != nil {
		fmt.Println("[启动器] ", err)
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

func parseScan() Config {
	var index int
	var times int

	fmt.Println("当前支持的地图: ")
	for i, o := range Options {
		fmt.Printf("%d. %s (%s)\n", i+1, o.Map, o.Mode)
	}
	for {
		fmt.Print("请选择目标地图(按下回车确认): ")
		fmt.Scanln(&index)
		if index <= 0 || index > len(Options) {
			fmt.Println("尚未支持目标地图")
			continue
		} else {
			break
		}
	}
	option := Options[index-1]

	for {
		fmt.Print("要进行的次数(默认:999): ")
		fmt.Scanln(&times)
		if times == 0 {
			times = option.Times
			break
		}
		if times < 1 || times > 999 {
			fmt.Println("请输入1~999范围内的次数")
			continue
		} else {
			break
		}
	}
	option.Times = times
	fmt.Printf("\n\n")

	return option
}

func showReadMe() {
	fmt.Println("声明: 本软件仅供个人学习使用, 代码已在Github开源。")
	fmt.Println("https://github.com/YinZe0/star-map-tool")
	fmt.Println("软件使用须知: ")
	fmt.Println("- 请通过Github下载已发布的exe程序,以防不明途径来源软件的病毒攻击")
	fmt.Println("- 由于需要调整游戏程序窗体大小,需右键以管理员身份运行")
	fmt.Println("- 请将游戏改为任意分辨率窗口化")
	fmt.Println("- 请关闭自动翻越障碍物")
	fmt.Println("- 设置自动攻击时,尽量不要设置长时间在场的幻想技能,可能会遮挡目标识别")
	fmt.Println("- 设置 > 画面 > 表现设置 (所有内容改为关闭、极简)")
	fmt.Println("- 设置 > 操控 > PC 操作设置 (镜头水平灵敏度、镜头垂直灵敏度=3)")
	fmt.Println("- 暂无画质要求 (推荐最低画质)")
	fmt.Println("补充: ")
	fmt.Println("- 程序完全基于图像识别进行, 使用时切换窗口会影响副本流程;")
	fmt.Println("- 程序使用时会占用键盘、鼠标, 使用期间自行操控可能遇到程序抢手现象;")
	fmt.Println("- 使用结束后, 请按正常流程退出本软件 (按下F10 -> 等待F10执行结束 -> 关闭命令窗口);")
	fmt.Println("- 退出本软件后, 如果遇到键盘的不合理行为, 可尝试逐个按下shift、ctrl、w、a、s、d解决;")
	fmt.Println("- 不同的副本有各自的职业要求, 请在选择地图后查看详情描述, 未按要求进行会降低刷图成功率;")
	fmt.Printf("\n\n")
}

func showMapDescripion(config Config) {
	description := "无要求"
	if len(config.Description) > 0 {
		description = config.Description
	}

	fmt.Printf("本地图需注意: %s\n\n", description)
}

func SetConsoleTitle(title string) {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		panic("获取窗口标题失败")
	}
	ret, _, _ := procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(titlePtr)))
	if ret == 0 {
		panic("修改窗口标题失败")
	}
}

func resizeCli() {
	hwnd := robotgo.FindWindow(Title)
	val := win.SetWindowPos(hwnd, win.HWND_TOP, 1280, 0, 1920-1280, 800, win.SWP_SHOWWINDOW)
	if !val {
		panic("调整当前窗口大小失败")
	}
}

func handlePanic() {
	if r := recover(); r != nil {
		game.ReleaseAllKey()
		fmt.Println("\n============ 异常捕获 ===============")
		fmt.Printf("异常信息: %v\n", r)

		fmt.Print("按任意键退出程序...")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
}
