package detector

import (
	"image"
	"image/color"

	"gocv.io/x/gocv"
)

type ColorDetectParam struct {
	Img            gocv.Mat
	MinColor       color.RGBA
	MaxColor       color.RGBA
	ScoreThreshold float64
}

type ColorDetector interface {
	Detect(param ColorDetectParam) ([]image.Rectangle, []float64, bool)
}

type ColorDetectorImpl struct {
}

func NewColorDetector() ColorDetector {
	return &ColorDetectorImpl{}
}

func NewColorDetectParam(img gocv.Mat, minColor color.RGBA, maxColor color.RGBA, scoreThreshold float32) ColorDetectParam {
	return ColorDetectParam{
		Img:            img,
		MinColor:       minColor,
		MaxColor:       maxColor,
		ScoreThreshold: float64(scoreThreshold),
	}
}

func (d *ColorDetectorImpl) Detect(param ColorDetectParam) ([]image.Rectangle, []float64, bool) {
	img := param.Img
	mincolor := param.MinColor
	maxcolor := param.MaxColor
	scoreThreshold := param.ScoreThreshold

	imgHsv := gocv.NewMat()
	mask := gocv.NewMat()
	defer imgHsv.Close()
	defer mask.Close()

	gocv.CvtColor(img, &imgHsv, gocv.ColorBGRToHSV)

	lower := gocv.NewScalar(float64(mincolor.R), float64(mincolor.G), float64(mincolor.B), 0) // HSV下限
	upper := gocv.NewScalar(float64(maxcolor.R), float64(maxcolor.G), float64(maxcolor.B), 0) // HSV上限
	gocv.InRangeWithScalar(imgHsv, lower, upper, &mask)

	contours := gocv.FindContours(mask, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	if contours.Size() == 0 {
		return nil, nil, false
	}

	var rectList []image.Rectangle
	var sizeList []float64
	for i := range contours.Size() {
		contour := contours.At(i)
		area := gocv.ContourArea(contour)
		if area <= scoreThreshold {
			continue
		}
		rect := gocv.BoundingRect(contour)
		rectList = append(rectList, rect)
		sizeList = append(sizeList, area)
	}

	if len(rectList) == 0 {
		return nil, nil, false
	}
	return rectList, sizeList, true
}
