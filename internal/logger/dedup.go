package logger

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DedupWriter wraps an io.Writer and collapses consecutive duplicate lines.
// When the same line is written N times in a row, only the first instance is
// passed through; subsequent duplicates are counted, and a summary like
// "[repeated 42×]" is emitted after a flush timeout or when a different line
// arrives.
type DedupWriter struct {
	mu       sync.Mutex
	w        io.Writer
	lastLine []byte
	count    int
	flushCh  chan struct{}
	done     chan struct{}
}

// NewDedupWriter creates a DedupWriter that flushes accumulated duplicate
// counts after 5 seconds of inactivity.
func NewDedupWriter(w io.Writer) *DedupWriter {
	dw := &DedupWriter{
		w:       w,
		flushCh: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	go dw.flushLoop()
	return dw
}

func (dw *DedupWriter) Write(p []byte) (int, error) {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	// Same line as previous? Count and suppress.
	if dw.lastLine != nil && bytes.Equal(dw.lastLine, p) {
		dw.count++
		// Signal flush timer
		select {
		case dw.flushCh <- struct{}{}:
		default:
		}
		return len(p), nil
	}

	// Different line: flush previous summary first
	dw.flushLocked()
	dw.lastLine = make([]byte, len(p))
	copy(dw.lastLine, p)
	dw.count = 0

	// Write the new line
	return dw.w.Write(p)
}

// Close flushes any pending summary and stops the background goroutine.
func (dw *DedupWriter) Close() error {
	close(dw.done)
	dw.mu.Lock()
	dw.flushLocked()
	dw.mu.Unlock()
	return nil
}

// flushLoop periodically flushes stale duplicate counts.
func (dw *DedupWriter) flushLoop() {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-dw.done:
			return
		case <-dw.flushCh:
			timer.Reset(5 * time.Second)
		case <-timer.C:
			dw.mu.Lock()
			dw.flushLocked()
			dw.mu.Unlock()
		}
	}
}

// flushLocked outputs the accumulated repeat summary if any.
// Formats as a proper log line so frontend log parser can handle it.
func (dw *DedupWriter) flushLocked() {
	if dw.lastLine != nil && dw.count > 0 {
		// Trim trailing newline, truncate, and escape quotes
		last := dw.lastLine
		if len(last) > 0 && last[len(last)-1] == '\n' {
			last = last[:len(last)-1]
		}
		lastStr := string(last)
		if len(lastStr) > 200 {
			lastStr = lastStr[:200] + "..."
		}
		lastStr = strings.ReplaceAll(lastStr, "\"", "'")
		summary := []byte("time=\"" + time.Now().Format("2006-01-02 15:04:05.000") + "\" level=WARN component=logger msg=\"重复日志折叠: 以下日志连续重复了 " + strconv.Itoa(dw.count) + " 次\" last_msg=\"" + lastStr + "\"\n")
		dw.w.Write(summary)
		dw.count = 0
	}
}
