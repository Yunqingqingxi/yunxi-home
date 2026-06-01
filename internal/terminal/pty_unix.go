//go:build !windows

package terminal

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type unixPty struct {
	f   *os.File
	cmd *exec.Cmd
}

func startShell() (ptyIO, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "bash"
	}
	cmd := exec.Command(shell)
	// Override HOME to an accessible directory (service may have ProtectHome=yes)
	cmd.Env = append(os.Environ(), "HOME=/opt/yunxi-home")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return nil, err
	}
	return &unixPty{f: f, cmd: cmd}, nil
}

func (p *unixPty) Read(buf []byte) (int, error)  { return p.f.Read(buf) }
func (p *unixPty) Write(data []byte) (int, error) { return p.f.Write(data) }

func (p *unixPty) Close() error {
	p.f.Close()
	return p.cmd.Process.Kill()
}

func (p *unixPty) Resize(rows, cols int) error {
	if rows > 0 && cols > 0 {
		return pty.Setsize(p.f, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	}
	return nil
}
