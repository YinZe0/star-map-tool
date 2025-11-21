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

func GetAngle(screenCenter image.Point, b image.Point, factor float32) int { // 14.2
	x1 := screenCenter.X
	x2 := b.X

	pixel := x2 - x1
	angle := float64(pixel) / float64(factor)
	return int(angle)
}
