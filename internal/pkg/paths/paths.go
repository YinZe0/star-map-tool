package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var (
	projectRoot string
)

func GetAbsolutePath(relativePath string) string {
	if projectRoot == "" {
		if err := initProjectRoot(); err != nil {
			panic(err)
		}
	}
	return filepath.Join(projectRoot, relativePath)
}

func initProjectRoot() error {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("无法获取调用者信息")
	}

	dir := filepath.Dir(filename)
	for i := 0; i < 10; i++ { // 最多回溯10层
		if isProjectRoot(dir) {
			projectRoot = dir
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return fmt.Errorf("未找到项目根目录")
}

func isProjectRoot(dir string) bool {
	markers := []string{"go.mod", ".git", "README.md", "project.root"}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}
