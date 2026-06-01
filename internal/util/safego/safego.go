// Package safego provides safe goroutine launching with automatic panic recovery.
package safego

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

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
// maxRestarts limits the number of restarts within restartWindow; 0 = unlimited.
func GoWithRestart(name string, maxRestarts int, restartWindow time.Duration, fn func()) {
	go func() {
		restarts := 0
		windowStart := time.Now()
		for {
			func() {
				defer recoverPanic(name)
				fn()
			}()
			// If fn() returns normally, exit the loop
			return
		loop:
			for {
				select {
				case <-time.After(time.Second):
					// This is unreachable in the normal flow but kept for structure
				default:
					break loop
				}
			}

			restarts++
			if maxRestarts > 0 && restarts > maxRestarts {
				slog.Error("safego: max restarts reached", "name", name, "restarts", restarts)
				return
			}
			if time.Since(windowStart) > restartWindow {
				restarts = 0
				windowStart = time.Now()
			}
			slog.Warn("safego: restarting goroutine", "name", name, "restarts", restarts)
			time.Sleep(time.Second) // brief pause before restart
		}
	}()
}

func recoverPanic(name string) {
	r := recover()
	if r == nil {
		return
	}
	stack := debug.Stack()
	slog.Error("safego: goroutine panic",
		"name", name,
		"panic", fmt.Sprintf("%v", r),
		"stack", string(stack),
	)
	if Notifier != nil {
		Notifier(name, r, stack)
	}
}
