package sleeper

import (
	"time"
)

// 目前未全面实现基于图形的目的检测，只能通过这种方式暂时使

func SleepBusyLoop(duration int) {
	start := time.Now()
	val := time.Duration(duration * int(time.Millisecond))

	// 让时间的大头使用sleep执行
	if val > 500*time.Millisecond {
		time.Sleep(val - 500*time.Millisecond)
	}
	if time.Since(start) > val {
		return
	}

	// 让500毫秒内的逻辑使用忙循环
	for time.Since(start) < val {
	}
}

func Sleep(duration int) {
	time.Sleep(time.Duration(duration) * time.Millisecond)
}
