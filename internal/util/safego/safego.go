// Package safego provides safe goroutine launching with automatic panic recovery.
package safego

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
)

var log = logger.ForComponent("runtime")

// Notifier is called when a goroutine panics. Set it to integrate with custom alerting.
var Notifier func(name string, panicVal any, stack []byte)

// Go launches a named goroutine with panic recovery.
func Go(name string, fn func()) {
	go func() {
		defer recoverPanic(name)
		fn()
	}()
}

// GoWithContext launches a named goroutine that inherits a context.
// Returns a child context that is cancelled when the goroutine exits.
func GoWithContext(parent context.Context, name string, fn func(ctx context.Context)) context.Context {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		defer cancel()
		defer recoverPanic(name)
		fn(ctx)
	}()
	return ctx
}

// GoWithRestart launches a named goroutine that restarts on panic (useful for long-running loops).
// If fn panics, it is restarted after a brief delay. If fn returns normally, the goroutine exits.
// maxRestarts limits the number of restarts within restartWindow; 0 = unlimited.
func GoWithRestart(name string, maxRestarts int, restartWindow time.Duration, fn func()) {
	go func() {
		restarts := 0
		windowStart := time.Now()
		for {
			panicked := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicked = true
						stack := debug.Stack()
						log.Error("goroutine panic (will restart)",
							logger.KeyEvent, "goroutine_panic",
							"name", name,
							"panic", fmt.Sprintf("%v", r),
							"stack", string(stack),
						)
						if Notifier != nil {
							Notifier(name, r, stack)
						}
					}
				}()
				fn()
			}()
			if !panicked {
				return // normal exit, don't restart
			}
			restarts++
			if maxRestarts > 0 && restarts > maxRestarts {
				log.Error("goroutine 重启次数超限", logger.KeyEvent, "goroutine_panic", "name", name, "restarts", restarts)
				return
			}
			if time.Since(windowStart) > restartWindow {
				restarts = 0
				windowStart = time.Now()
			}
			log.Warn("重启 goroutine", logger.KeyEvent, "goroutine_restart", "name", name, "restarts", restarts)
			time.Sleep(time.Second)
		}
	}()
}

func recoverPanic(name string) {
	r := recover()
	if r == nil {
		return
	}
	stack := debug.Stack()
	log.Error("goroutine panic",
		logger.KeyEvent, "goroutine_panic",
		"name", name,
		"panic", fmt.Sprintf("%v", r),
		"stack", string(stack),
	)
	if Notifier != nil {
		Notifier(name, r, stack)
	}
}
