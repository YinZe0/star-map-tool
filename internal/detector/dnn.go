package detector

import (
	"image"

	"gocv.io/x/gocv"
)

type DNNDetectParam struct {
	Img            *gocv.Mat
	ScoreThreshold float32
	NMSThreshold   float32
}

type DNNDetector interface {
	Detect(param *DNNDetectParam, filter ...int) ([]image.Rectangle, []int, bool)
}

type DNNDetectorImpl struct {
	modePath string
}

func NewDNNDetector(modePath string) DNNDetector {
	return &DNNDetectorImpl{
		modePath: modePath,
	}
}

func NewDNNDetectParam(img *gocv.Mat, scoreThreshold float32, nmsThreshold float32) *DNNDetectParam {
	return &DNNDetectParam{
		Img:            img,
		ScoreThreshold: scoreThreshold,
		NMSThreshold:   nmsThreshold,
	}
}

func (d *DNNDetectorImpl) Detect(param *DNNDetectParam, filter ...int) ([]image.Rectangle, []int, bool) {
	modePath := d.modePath
	img := param.Img
	scoreThreshold := param.ScoreThreshold // 当前测试用的0.4
	nmsThreshold := param.NMSThreshold     // 当前测试用的0.45

	// 加载模型
	net := gocv.ReadNetFromONNX(modePath)
	if net.Empty() {
		panic("Error loading ONNX model")
	}
	net.SetPreferableBackend(gocv.NetBackendDefault)
	net.SetPreferableTarget(gocv.NetTargetCPU)
	defer net.Close()

	// 图像转为模型需要的形式
	blob := gocv.BlobFromImage(*img, 1.0/255.0, image.Pt(1024, 1024), gocv.NewScalar(0, 0, 0, 0), true, false)
	defer blob.Close()
	net.SetInput(blob, "")

	// 推理
	outputNames := getOutputNames(&net)
	if len(outputNames) == 0 {
		return nil, nil, false
	}
	outs := net.ForwardLayers(outputNames) // 张量集合
	defer func() {
		for _, out := range outs {
			out.Close()
		}
	}()

	boxes, confidences, classIds := performDetection(&outs, img.Cols(), img.Rows(), scoreThreshold)
	if len(boxes) == 0 {
		return nil, nil, false
	}
	// NMS
	indices := gocv.NMSBoxes(boxes, confidences, scoreThreshold, nmsThreshold)

	m := make(map[int]bool)
	for _, v := range filter {
		m[v] = true
	}

	var targetBoxList []image.Rectangle
	var classIdList []int
	filterLength := len(filter)
	for _, idx := range indices {
		if idx == 0 {
			continue
		}

		classId := classIds[idx]
		if filterLength > 0 && !m[classId] { // 用户要求获取指定classId的数据
			continue
		}
		targetBoxList = append(targetBoxList, boxes[idx])
		classIdList = append(classIdList, classIds[idx])
	}
	return targetBoxList, classIdList, len(classIdList) > 0
}

func performDetection(outs *[]gocv.Mat, imgW, imgH int, scoreThreshold float32) ([]image.Rectangle, []float32, []int) {
	var boxes []image.Rectangle
	var confidences []float32
	var classIds []int

	const inputSize float32 = 1024 // 当前模型训练尺寸是1024
	for _, out := range *outs {
		gocv.TransposeND(out, []int{0, 2, 1}, &out)
		out = out.Reshape(1, out.Size()[1])
		defer out.Close()

		scaleX := float32(imgW) / inputSize
		scaleY := float32(imgH) / inputSize
		for i := 0; i < out.Rows(); i++ {
			confidence, classId := getScoreAndClassId(&out, i)
			if confidence < scoreThreshold {
				continue
			}

			centerX := out.GetFloatAt(i, 0)
			centerY := out.GetFloatAt(i, 1)
			w := out.GetFloatAt(i, 2)
			h := out.GetFloatAt(i, 3)

			left := (centerX - w/2) * scaleX
			top := (centerY - h/2) * scaleY
			right := (centerX + w/2) * scaleX
			bottom := (centerY + h/2) * scaleY

			boxes = append(boxes, image.Rect(int(left), int(top), int(right), int(bottom)))

			confidences = append(confidences, confidence)
			classIds = append(classIds, classId)
		}
	}
	return boxes, confidences, classIds
}

func getOutputNames(net *gocv.Net) []string {
	var outputLayers []string
	for _, i := range net.GetUnconnectedOutLayers() {
		layer := net.GetLayer(i)
		layerName := layer.GetName()
		if layerName != "_input" {
			outputLayers = append(outputLayers, layerName)
		}
	}

	return outputLayers
}

func getScoreAndClassId(out *gocv.Mat, row int) (float32, int) {
	// x, y, w, h, class1_score, class2_score, class3_score...

	cols := out.Cols()
	if cols < 5 {
		// Yolo 不可能返回小于5的列数
		return 0, -1
	} else if cols == 5 {
		return out.GetFloatAt(row, 4), 0
	}

	classId := 0
	maxScore := out.GetFloatAt(row, 4)
	for i := 5; i < cols; i++ {
		score := out.GetFloatAt(row, i)
		if score > maxScore {
			maxScore = score
			classId = i - 4
		}
	}
	return maxScore, classId
}
