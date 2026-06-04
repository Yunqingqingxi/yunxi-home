package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"os"
	"os/exec"
	"sync"
	"time"
)

// Transport 与 MCP 服务器的 stdio 通信通道
type Transport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	mu     sync.Mutex
}

// StartTransport 启动一个 MCP 服务器进程并通过 stdio 连接
func StartTransport(command string, args []string, env map[string]string) (*Transport, error) {
	cmd := exec.Command(command, args...)
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", command, err)
	}

	scanner := bufio.NewScanner(stdoutPipe)
	// MCP servers can return large results (e.g., file_search with 500+ results).
	// Default 64KB buffer is too small; use 4MB to handle large JSON-RPC responses.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	return &Transport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: scanner,
	}, nil
}

// Send 发送一条 JSON-RPC 请求并等待响应。
// 兼容输出多行日志的 MCP 服务器：跳过非 JSON 行，直到找到有效的 JSON-RPC 响应。
func (t *Transport) Send(request map[string]any) (map[string]any, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// 读取直到找到有效的 JSON-RPC 响应（跳过非 JSON 日志行）
	deadline := time.After(30 * time.Second)
	for {
		select {
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for MCP response")
		default:
		}

		if !t.stdout.Scan() {
			if err := t.stdout.Err(); err != nil {
				return nil, fmt.Errorf("read response: %w", err)
			}
			return nil, fmt.Errorf("unexpected EOF from MCP server")
		}

		line := t.stdout.Bytes()
		if len(line) == 0 {
			continue
		}
		// 跳过非 JSON 行（日志、warning 等）
		if line[0] != '{' {
			log.Debug("MCP transport: skipping non-JSON line", "line", string(line)[:min(len(line), 100)])
			continue
		}

		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			log.Debug("MCP transport: skipping unparseable line", "line", string(line)[:min(len(line), 100)], "error", err)
			continue
		}

		// 确认是 JSON-RPC 响应（有 jsonrpc 字段或有 id 字段）
		if _, hasRPC := response["jsonrpc"]; !hasRPC {
			if _, hasID := response["id"]; !hasID {
				continue
			}
		}

		if errObj, ok := response["error"]; ok {
			return nil, fmt.Errorf("MCP error: %v", errObj)
		}

		return response, nil
	}
}

// SendNotification 发送一条 JSON-RPC 通知（无 id，不等待响应）
func (t *Transport) SendNotification(method string, params map[string]any) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	notif := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		notif["params"] = params
	}

	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	if _, err := t.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write notification: %w", err)
	}

	return nil
}

// Close 终止 MCP 服务器进程
func (t *Transport) Close() error {
	t.stdin.Close()
	return t.cmd.Wait()
}
