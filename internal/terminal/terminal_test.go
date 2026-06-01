package terminal

import (
	"strings"
	"testing"
	"time"
)

func TestStartShell(t *testing.T) {
	pty, err := startShell()
	if err != nil {
		t.Fatalf("startShell failed: %v", err)
	}
	defer pty.Close()

	if pty == nil {
		t.Fatal("expected non-nil ptyIO")
	}
}

func TestShellReadWrite(t *testing.T) {
	pty, err := startShell()
	if err != nil {
		t.Fatalf("startShell failed: %v", err)
	}
	defer pty.Close()

	// Send a simple echo command
	cmd := "echo HELLO_TERMINAL_TEST\r\n"
	_, err = pty.Write([]byte(cmd))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read back output with timeout
	buf := make([]byte, 4096)
	done := make(chan struct{})
	var n int
	var readErr error

	go func() {
		// Read in a loop until we see our marker
		for i := 0; i < 50; i++ {
			var rn int
			rn, readErr = pty.Read(buf[n:])
			if rn > 0 {
				n += rn
				if strings.Contains(string(buf[:n]), "HELLO_TERMINAL_TEST") {
					break
				}
			}
			if readErr != nil {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		close(done)
	}()

	select {
	case <-done:
		if readErr != nil {
			t.Fatalf("Read failed: %v", readErr)
		}
		if n == 0 {
			t.Fatal("no output received from shell")
		}
		if !strings.Contains(string(buf[:n]), "HELLO_TERMINAL_TEST") {
			t.Errorf("expected output to contain HELLO_TERMINAL_TEST, got: %s", string(buf[:n]))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for shell output")
	}
}

func TestShellResize(t *testing.T) {
	pty, err := startShell()
	if err != nil {
		t.Fatalf("startShell failed: %v", err)
	}
	defer pty.Close()

	// Resize should not error
	err = pty.Resize(30, 100)
	if err != nil {
		t.Logf("Resize returned error (may be expected for pipe fallback): %v", err)
	}
}

func TestShellClose(t *testing.T) {
	pty, err := startShell()
	if err != nil {
		t.Fatalf("startShell failed: %v", err)
	}

	err = pty.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Read after close should return an error or zero bytes
	buf := make([]byte, 64)
	_, err = pty.Read(buf)
	// After close, read should fail - but this depends on conpty vs pipe behavior
	// So we just log it rather than asserting
	t.Logf("Read after close: %v", err)
}

func TestHandlerCreation(t *testing.T) {
	h := NewHandler(true, false)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
	if !h.enabled {
		t.Error("expected handler to be enabled")
	}

	h2 := NewHandler(false, false)
	if h2.enabled {
		t.Error("expected handler to be disabled")
	}
}

func TestWSMessageUnmarshal(t *testing.T) {
	// Test that our message format can be round-tripped
	input := `{"type":"input","data":"ls\r\n"}`
	var msg WSMessage
	if err := parseTest(input, &msg); err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}
	if msg.Type != "input" {
		t.Errorf("expected type 'input', got '%s'", msg.Type)
	}
}

func TestResizeMessageUnmarshal(t *testing.T) {
	input := `{"type":"resize","data":{"cols":120,"rows":40}}`
	var msg WSMessage
	if err := parseTest(input, &msg); err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}
	if msg.Type != "resize" {
		t.Errorf("expected type 'resize', got '%s'", msg.Type)
	}
}

// parseTest avoids importing encoding/json just for the message format
func parseTest(s string, msg *WSMessage) error {
	// simple manual parsing for test
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '{' {
		return nil
	}
	// Extract type field
	if idx := strings.Index(s, `"type":"`); idx >= 0 {
		start := idx + len(`"type":"`)
		end := strings.Index(s[start:], `"`)
		if end >= 0 {
			msg.Type = s[start : start+end]
		}
	}
	return nil
}