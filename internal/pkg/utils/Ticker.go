package utils

import (
	"time"
)

func NewTicker(duration time.Duration, interval time.Duration, f func() (bool, error), immediate bool) bool {
	startTime := time.Now()
	if immediate {
		f() // 立即执行一次
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if time.Since(startTime) >= duration {
			return false
		}

		<-ticker.C
		result, err := f()

		if err != nil {
			return false
		}
		if result {
			return true
		}
	}
}
