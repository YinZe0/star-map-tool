package game

import (
	"errors"
	"image"
	"log"
	"time"

	"github.com/go-vgo/robotgo"
	"github.com/tailscale/win"
	"gocv.io/x/gocv"
)

type Game struct {
	Pid   int
	Name  string
	Title string
	Rect  *GameRect // 游戏窗口位置
}

// 原点(0, 0)是屏幕左上角
type GameRect struct {
	x int
	y int
	w int
	h int
}

var MoveKeys = []string{"w", "a", "s", "d", "ctrl", "shift"}

/**
 * @param name 游戏的进程名称，例如：Star.exe
 */
func NewGame(name string, title string) (*Game, error) {
	if len(name) == 0 {
		return nil, errors.New("游戏进程名称不能为空")
	}

	g := &Game{Name: name, Title: title}
	return g, nil
}

func (g *Game) Initialize() bool {
	robotgo.MouseSleep = 200
	robotgo.KeySleep = 100
	robotgo.Process()

	var width int32 = 1280
	var height int32 = 800

	_, err := g.GetPid()
	if err != nil {
		return false
	}
	log.Printf("[初始器] 当前游戏窗口标题:%s 进程ID:%d 进程名称:%s\n", g.Title, g.Pid, g.Name)

	g.Active()
	time.Sleep(time.Duration(1) * time.Second)
	g.GetRect()
	if _, err := g.Resize(width, height); err != nil {
		log.Printf("[初始器] %s\n", err.Error())
		return false
	}
	return true
}

func (g *Game) Active() {
	robotgo.ActivePid(g.Pid)
}

func (g *Game) GetPid() (int, error) {
	if g.Pid != 0 {
		return g.Pid, nil
	}

	pidList, err := robotgo.FindIds(g.Name)
	if err != nil || len(pidList) <= 0 {
		log.Println("[初始器] 未发现目标游戏进程, 请启动游戏后重试!")
		return -1, errors.New("未发现目标游戏进程")
	}

	pid := pidList[0]
	g.Pid = pid

	return pid, nil
}

func (g *Game) GetRect() (*GameRect, error) {
	if g.Rect != nil {
		return g.Rect, nil
	}
	x, y, w, h := robotgo.GetBounds(g.Pid)

	rect := &GameRect{
		x: x, y: y, w: w, h: h,
	}
	g.Rect = rect
	return rect, nil
}

func (g *Game) GetScreenshot(args ...int) *image.Image {
	length := len(args)
	if !(length == 0 || length == 4) {
		panic("参数数量错误!")
	}

	rect := g.Rect
	var bitmap robotgo.CBitmap
	if length == 0 {
		bitmap = robotgo.CaptureScreen(rect.x, rect.y, rect.w, rect.h)
	} else {
		bitmap = robotgo.CaptureScreen(args[0], args[1], args[2], args[3])
	}

	defer robotgo.FreeBitmap(bitmap)
	image := robotgo.ToImage(bitmap)
	return &image
}

func (g *Game) GetScreenshotMatRGB(args ...int) (*gocv.Mat, error) {
	image := g.GetScreenshot(args...)

	mat, err := gocv.ImageToMatRGB(*image) // 调用层必须要关闭，不然会内存泄露
	return &mat, err
}

func (g *Game) Resize(w int32, h int32) (bool, error) {
	screenWidth, screenHeight := robotgo.GetScreenSize()
	rect := g.Rect
	log.Printf("[初始器] 当前屏幕分辨率: %d x %d 游戏窗口大小: %d x %d\n", screenWidth, screenHeight, rect.w, rect.h)

	hwnd := robotgo.FindWindow(g.Title)
	val := win.SetWindowPos(hwnd, win.HWND_TOP, 0, 0, w, h, win.SWP_SHOWWINDOW)
	if !val {
		return false, errors.New("调整窗口大小失败")
	}

	g.refreshRect()

	rect = g.Rect
	log.Printf("[初始器] 变更后游戏窗口大小: %d x %d\n", rect.w, rect.h)
	return val, nil
}

func (g *Game) ReleaseAllKey() {
	for _, key := range MoveKeys {
		robotgo.KeyUp(key)
	}
}

func (g *Game) refreshRect() error {
	g.Rect = nil
	g.GetRect()

	return nil
}
