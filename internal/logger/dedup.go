package logger

import (
	"io"
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
	if dw.lastLine != nil && bytesEqual(dw.lastLine, p) {
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

// flushLocked outputs the accumulated "[repeated N×]" summary if any.
// Must be called with dw.mu held.
func (dw *DedupWriter) flushLocked() {
	if dw.lastLine != nil && dw.count > 0 {
		summary := []byte("[repeated " + formatInt(dw.count) + "×]\n")
		dw.w.Write(summary)
		dw.count = 0
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
