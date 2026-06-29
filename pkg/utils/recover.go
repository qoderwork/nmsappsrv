package utils

import (
	"fmt"
	"runtime/debug"
	"time"

	"nmsappsrv/pkg/logger"
)

// SafeGo 在 recover 保护下运行 goroutine
func SafeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("[PANIC-RECOVER] %s: %v\n%s", name, r, debug.Stack())
			}
		}()
		fn()
	}()
}

// SafeGoWithRetry 在 recover 保护下运行 goroutine，panic 后自动重试
func SafeGoWithRetry(name string, fn func(), retryInterval ...int) {
	interval := 5
	if len(retryInterval) > 0 && retryInterval[0] > 0 {
		interval = retryInterval[0]
	}

	go func() {
		for {
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Errorf("[PANIC-RECOVER] %s: %v\n%s", name, r, debug.Stack())
					}
				}()
				fn()
			}()

			logger.Infof("[RESTART] %s restarting in %ds...", name, interval)
			<-time.After(time.Duration(interval) * time.Second)
		}
	}()
}

// SafeWrap 包装函数，添加 recover 保护
func SafeWrap(name string, fn func()) func() {
	return func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("[PANIC-RECOVER] %s: %v\n%s", name, r, debug.Stack())
			}
		}()
		fn()
	}
}

// Must 断言无错误，有错误则 panic
func Must(err error, msg ...string) {
	if err != nil {
		if len(msg) > 0 {
			panic(fmt.Sprintf("%s: %v", msg[0], err))
		}
		panic(err)
	}
}
