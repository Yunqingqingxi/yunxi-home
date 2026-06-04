//go:build windows

package terminal

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/UserExistsError/conpty"
)

// conPtyWrapper wraps a Windows ConPTY pseudo-console.
// Falls back to pipe-based shell if ConPTY is unavailable.
type conPtyWrapper struct {
	cpty *conpty.ConPty

	// fallback fields
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func startShell() (ptyIO, error) {
	// Try ConPTY first for proper terminal emulation
	c, err := conpty.Start("powershell.exe -NoLogo",
		conpty.ConPtyDimensions(120, 30),
	)
	if err == nil {
		return &conPtyWrapper{cpty: c}, nil
	}

	// Fallback: plain pipe-based PowerShell (no PTY, limited escape support)
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoExit")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("startShell stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("startShell stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("startShell: %w", err)
	}
	return &conPtyWrapper{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

func (w *conPtyWrapper) Read(buf []byte) (int, error) {
	if w.cpty != nil {
		return w.cpty.Read(buf)
	}
	return w.stdout.Read(buf)
}

func (w *conPtyWrapper) Write(data []byte) (int, error) {
	if w.cpty != nil {
		return w.cpty.Write(data)
	}
	return w.stdin.Write(data)
}

func (w *conPtyWrapper) Close() error {
	if w.cpty != nil {
		return w.cpty.Close()
	}
	w.stdin.Close()
	if w.cmd.Process != nil {
		w.cmd.Process.Kill()
	}
	return nil
}

func (w *conPtyWrapper) Resize(rows, cols int) error {
	if rows <= 0 || cols <= 0 {
		return nil
	}
	if w.cpty != nil {
		return w.cpty.Resize(cols, rows)
	}
	// Fallback: send resize via stdin (may interfere with running commands)
	psCmd := fmt.Sprintf(
		"$host.UI.RawUI.WindowSize = New-Object System.Management.Automation.Host.Size(%d,%d); $host.UI.RawUI.BufferSize = New-Object System.Management.Automation.Host.Size(%d,%d)",
		cols, rows, cols, 3000,
	)
	w.stdin.Write([]byte(psCmd + "\r\n"))
	return nil
}

var _ ptyIO = (*conPtyWrapper)(nil)