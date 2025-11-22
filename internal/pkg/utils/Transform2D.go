package utils

import "image"

func GetCenter(rect image.Rectangle) image.Point {
	centerX := rect.Min.X + (rect.Max.X-rect.Min.X)/2
	centerY := rect.Min.Y + (rect.Max.Y-rect.Min.Y)/2
	return image.Point{X: centerX, Y: centerY}
}

func ToGlobalPoint(offset image.Point, local image.Point) image.Point {
	return image.Point{X: offset.X + local.X, Y: offset.Y + local.Y}
}

func GetRectSize(minX int, minY int, maxX int, maxY int) (int, int) {
	return maxX - minX, maxY - minY
}

// 获取游戏窗口内识别到的物体 与 屏幕正中间的 角度差
// factor: 14.2 是一个经验值, 代表两个点之间每14.2像素为角度1
func GetAngle(screenCenter image.Point, areaCenter image.Point, factor float32) int { // 14.2
	x1 := screenCenter.X
	x2 := areaCenter.X

	pixel := x2 - x1
	angle := float64(pixel) / float64(factor)
	return int(angle)
}

func MergeRectangles(rectList []image.Rectangle) image.Rectangle {
	// 这里传进来的是切片，值传递代价很小
	if len(rectList) == 0 {
		return image.Rectangle{}
	}

	minX := rectList[0].Min.X
	minY := rectList[0].Min.Y
	maxX := rectList[0].Max.X
	maxY := rectList[0].Max.Y

	for _, rect := range rectList {
		if rect.Min.X < minX {
			minX = rect.Min.X
		}
		if rect.Min.Y < minY {
			minY = rect.Min.Y
		}
		if rect.Max.X > maxX {
			maxX = rect.Max.X
		}
		if rect.Max.Y > maxY {
			maxY = rect.Max.Y
		}
	}
	return image.Rect(minX, minY, maxX, maxY)
}
