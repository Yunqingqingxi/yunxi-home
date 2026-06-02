package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"sync"
	"time"
)

// ServerConfig 单个 MCP 服务器的配置
type ServerConfig struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"` // 环境变量（如 GITHUB_TOKEN, API_KEY 等）
}

// MCPTool 从 tools/list 返回的工具定义
type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Client 管理到单个 MCP 服务器的连接
type Client struct {
	cfg       ServerConfig
	transport *Transport
	tools     []MCPTool
	mu        sync.Mutex
	connected bool
}

// Connect 启动服务器并发现工具
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	t, err := StartTransport(c.cfg.Command, c.cfg.Args, c.cfg.Env)
	if err != nil {
		return fmt.Errorf("mcp %s: %w", c.cfg.Name, err)
	}
	c.transport = t

	// 发送 initialize 请求
	initResp, err := t.Send(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]string{
				"name":    "yunxi-home",
				"version": "1.0.0",
			},
		},
	})
	if err != nil {
		t.Close()
		return fmt.Errorf("mcp %s initialize: %w", c.cfg.Name, err)
	}
	_ = initResp

	// 发送 initialized 通知（通知无 id，不等待响应）
	if err := t.SendNotification("notifications/initialized", nil); err != nil {
		t.Close()
		return fmt.Errorf("mcp %s initialized notification: %w", c.cfg.Name, err)
	}

	// 获取工具列表
	listResp, err := t.Send(map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	})
	if err != nil {
		t.Close()
		return fmt.Errorf("mcp %s tools/list: %w", c.cfg.Name, err)
	}

	result, ok := listResp["result"].(map[string]any)
	if !ok {
		t.Close()
		return fmt.Errorf("mcp %s: unexpected tools/list response", c.cfg.Name)
	}

	toolsRaw, ok := result["tools"]
	if !ok {
		c.tools = nil
	} else {
		toolsJSON, _ := json.Marshal(toolsRaw)
		var tools []MCPTool
		if err := json.Unmarshal(toolsJSON, &tools); err != nil {
			t.Close()
			return fmt.Errorf("mcp %s: parse tools: %w", c.cfg.Name, err)
		}
		c.tools = tools
	}

	c.connected = true
	log.Info("MCP server connected", "name", c.cfg.Name, "tools", len(c.tools))
	return nil
}

// Tools 返回已发现的工具列表
func (c *Client) Tools() []MCPTool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tools
}

// CallTool 调用 MCP 服务器上的一个工具
func (c *Client) CallTool(ctx context.Context, toolName string, args map[string]any) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return "", fmt.Errorf("mcp %s: not connected", c.cfg.Name)
	}

	resp, err := c.transport.Send(map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": args,
		},
	})
	if err != nil {
		return "", fmt.Errorf("mcp %s call %s: %w", c.cfg.Name, toolName, err)
	}

	// 提取内容作为字符串
	result, _ := resp["result"].(map[string]any)
	if result == nil {
		return "", fmt.Errorf("mcp %s call %s: empty result", c.cfg.Name, toolName)
	}

	content, _ := result["content"].([]any)
	if content == nil {
		// 回退：将整个结果序列化为 JSON
		data, _ := json.Marshal(result)
		return string(data), nil
	}

	var text string
	for _, c := range content {
		if block, ok := c.(map[string]any); ok {
			if t, ok := block["text"].(string); ok {
				text += t
			}
		}
	}
	if text == "" {
		data, _ := json.Marshal(result)
		return string(data), nil
	}
	return text, nil
}

// Close 断开连接并终止服务器进程
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	if c.transport != nil {
		return c.transport.Close()
	}
	return nil
}

// Name 返回配置名称
func (c *Client) Name() string { return c.cfg.Name }

// IsConnected returns whether the client is currently connected.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// ── Manager ──────────────────────────────────────────────────

// Manager 管理多个 MCP 客户端
type Manager struct {
	clients map[string]*Client
	mu      sync.Mutex
}

// NewManager 创建一个 Manager
func NewManager() *Manager {
	return &Manager{clients: make(map[string]*Client)}
}

// AddServer 添加一个 MCP 服务器配置（尚未连接）
func (m *Manager) AddServer(cfg ServerConfig) *Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := &Client{cfg: cfg}
	m.clients[cfg.Name] = c
	return c
}

// ConnectAll 连接所有已配置的服务器（异步+超时+自动重试，不阻塞启动）
func (m *Manager) ConnectAll() error {
	m.mu.Lock()
	clients := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.mu.Unlock()

	for _, c := range clients {
		go func(client *Client) {
			// 首次尝试：30s 超时（npx 首次需要下载包）
			done := make(chan error, 1)
			go func() { done <- client.Connect() }()
			select {
			case err := <-done:
				if err != nil {
					log.Warn("MCP server connection failed, retrying in background", "name", client.Name(), "error", err)
					// 后台重试 2 次
					go m.retryConnect(client, 2)
				} else {
					log.Info("MCP server connected", "name", client.Name(), "tools", len(client.Tools()))
				}
			case <-time.After(30 * time.Second):
				log.Warn("MCP server connection timeout, will retry in background", "name", client.Name())
				go m.retryConnect(client, 2)
			}
		}(c)
	}
	return nil
}

// retryConnect 在后台重试连接
func (m *Manager) retryConnect(client *Client, maxRetries int) {
	for i := 0; i < maxRetries; i++ {
		time.Sleep(time.Duration(10+i*10) * time.Second)
		done := make(chan error, 1)
		go func() { done <- client.Connect() }()
		select {
		case err := <-done:
			if err == nil {
				log.Info("MCP server reconnected successfully", "name", client.Name(), "tools", len(client.Tools()))
				// 重连成功后需要重新注册工具（通过回调）
				return
			}
			log.Warn("MCP server retry failed", "name", client.Name(), "attempt", i+1, "error", err)
		case <-time.After(25 * time.Second):
			log.Warn("MCP server retry timeout", "name", client.Name(), "attempt", i+1)
		}
	}
	log.Error("MCP server connection failed after all retries", "name", client.Name())
}

// AllTools 返回所有已连接服务器的全部工具
func (m *Manager) AllTools() []struct {
	Server string
	Tool   MCPTool
} {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []struct {
		Server string
		Tool   MCPTool
	}
	for _, c := range m.clients {
		for _, t := range c.Tools() {
			result = append(result, struct {
				Server string
				Tool   MCPTool
			}{Server: c.Name(), Tool: t})
		}
	}
	return result
}

// GetClient 根据名称获取客户端
func (m *Manager) GetClient(name string) *Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clients[name]
}

// RemoveServer removes and closes a server by name.
func (m *Manager) RemoveServer(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.clients[name]; ok {
		done := make(chan struct{}, 1)
		go func() { c.Close(); done <- struct{}{} }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			log.Warn("MCP close timeout during remove", "name", name)
		}
		delete(m.clients, name)
	}
}

// CloseAll 关闭所有连接（每个 client 有 5s 超时保护）
func (m *Manager) CloseAll() {
	m.mu.Lock()
	clients := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.mu.Unlock()

	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func(client *Client) {
			defer wg.Done()
			done := make(chan struct{}, 1)
			go func() { client.Close(); done <- struct{}{} }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				log.Warn("MCP close timeout, forced", "name", client.Name())
			}
		}(c)
	}
	wg.Wait()
}
