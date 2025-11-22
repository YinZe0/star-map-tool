package detector

import (
	"image"
	"log"
	"path/filepath"

	"gocv.io/x/gocv"
)

type TemplateDetectParam struct {
	Img            gocv.Mat
	TemplateName   string
	ScoreThreshold float32
}

type TemplateDetector interface {
	Detect(param *TemplateDetectParam) (*image.Rectangle, bool)
}

type TemplateDetectorImpl struct {
	templates map[string]gocv.Mat
}

// 不推荐使用模板匹配，非常容易收到各类因素影响而无法识别（比较标准颜色、尺寸的图标可以使用局部截取后识别）
func NewTemplateDetector(templateDir string) TemplateDetector {
	d := &TemplateDetectorImpl{
		templates: make(map[string]gocv.Mat),
	}
	loadTemplates(d, templateDir)
	return d
}

func NewTemplateDetectParam(img gocv.Mat, templateName string, scoreThreshold float32) *TemplateDetectParam {
	return &TemplateDetectParam{
		Img:            img,
		TemplateName:   templateName,
		ScoreThreshold: scoreThreshold,
	}
}

func (d *TemplateDetectorImpl) Detect(param *TemplateDetectParam) (*image.Rectangle, bool) {
	templateName := param.TemplateName
	img := param.Img
	scoreThreshold := param.ScoreThreshold

	result := gocv.NewMat()
	defer result.Close()

	template := getTemplateByName(d, templateName)
	gocv.MatchTemplate(img, template, &result, gocv.TmCcoeffNormed, gocv.NewMat())

	_, maxVal, _, maxLoc := gocv.MinMaxLoc(result)
	if maxVal < scoreThreshold {
		return nil, false
	}

	rect := image.Rect(
		maxLoc.X,
		maxLoc.Y,
		maxLoc.X+template.Cols(),
		maxLoc.Y+template.Rows(),
	)
	return &rect, true
}

func loadTemplates(d *TemplateDetectorImpl, templateDir string) {
	files, err := filepath.Glob(filepath.Join(templateDir, "*.png"))
	if err != nil {
		log.Printf("[匹配器] 读取图片模板文件失败: %v\n", err)
		panic(err)
	}

	for _, file := range files {
		template := gocv.IMRead(file, gocv.IMReadColor)
		if template.Empty() {
			panic("[匹配器] 无法加载图片模板: " + file)
		}

		name := filepath.Base(file)
		d.templates[name] = template
	}
}

func getTemplateByName(d *TemplateDetectorImpl, name string) gocv.Mat {
	template, ok := d.templates[name]
	if !ok {
		panic("[匹配器] 未找到对应图片模板!")
	}
	return template
}
