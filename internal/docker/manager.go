package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"strings"
	"time"
)

var log = logger.ForComponent("docker")

// ContainerInfo 容器信息
type ContainerInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Status    string    `json:"status"`
	State     string    `json:"state"` // running, exited, paused
	Ports     string    `json:"ports,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// ContainerDetail 容器详情
type ContainerDetail struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	State   string            `json:"state"`
	Status  string            `json:"status"`
	Ports   []string          `json:"ports,omitempty"`
	Env     []string          `json:"env,omitempty"`
	Mounts  []string          `json:"mounts,omitempty"`
	Network string            `json:"network,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
}

// ComposeService Docker Compose 服务信息
type ComposeService struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Ports   string `json:"ports,omitempty"`
}

// Manager Docker 管理器
type Manager struct {
	enabled bool
}

// New 创建 Docker 管理器
func New(enabled bool) *Manager {
	return &Manager{enabled: enabled}
}

// IsAvailable 检查 Docker 是否可用
func (m *Manager) IsAvailable() bool {
	if !m.enabled {
		return false
	}
	_, err := exec.LookPath("docker")
	return err == nil
}

// ListContainers 列出所有容器
func (m *Manager) ListContainers(ctx context.Context, all bool) ([]ContainerInfo, error) {
	log.Info("查询Docker容器", "显示全部", all)
	if !m.IsAvailable() {
		return nil, fmt.Errorf("Docker 不可用")
	}
	args := []string{"ps", "--format", "{{json .}}"}
	if all {
		args = append(args, "-a")
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps 失败: %w", err)
	}

	var containers []ContainerInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var raw map[string]string
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		ci := ContainerInfo{
			ID:     raw["ID"],
			Name:   raw["Names"],
			Image:  raw["Image"],
			Status: raw["Status"],
			State:  raw["State"],
			Ports:  raw["Ports"],
		}
		if t, err := time.Parse("2006-01-02 15:04:05 -0700 MST", raw["CreatedAt"]); err == nil {
			ci.CreatedAt = t
		}
		containers = append(containers, ci)
	}
	return containers, nil
}

// ContainerAction 执行容器操作
func (m *Manager) ContainerAction(ctx context.Context, name, action string) (string, error) {
	if !m.IsAvailable() {
		return "", fmt.Errorf("Docker 不可用")
	}
	allowed := map[string]bool{
		"start": true, "stop": true, "restart": true,
		"pause": true, "unpause": true,
	}
	if !allowed[action] {
		return "", fmt.Errorf("不支持的操作: %s", action)
	}

	cmd := exec.CommandContext(ctx, "docker", action, name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("docker %s %s 失败: %w", action, name, err)
	}
	return string(out), nil
}

// GetLogs 获取容器日志
func (m *Manager) GetLogs(ctx context.Context, name string, tail int) (string, error) {
	if !m.IsAvailable() {
		return "", fmt.Errorf("Docker 不可用")
	}
	args := []string{"logs", "--tail", fmt.Sprintf("%d", tail), name}
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("获取日志失败: %w", err)
	}
	return string(out), nil
}

// ComposeAction Docker Compose 操作
func (m *Manager) ComposeAction(ctx context.Context, projectDir, action string) (string, error) {
	if !m.IsAvailable() {
		return "", fmt.Errorf("Docker 不可用")
	}
	_, err := exec.LookPath("docker-compose")
	if err != nil {
		// Try "docker compose" (v2)
		_, err = exec.LookPath("docker")
		if err != nil {
			return "", fmt.Errorf("docker compose 不可用")
		}
	}

	allowed := map[string]bool{"up": true, "down": true, "restart": true, "ps": true, "pull": true}
	if !allowed[action] {
		return "", fmt.Errorf("不支持的操作: %s", action)
	}

	var cmd *exec.Cmd
	if action == "ps" {
		cmd = exec.CommandContext(ctx, "docker", "compose", "-f", projectDir+"/docker-compose.yml", action, "--format", "json")
	} else if action == "up" {
		cmd = exec.CommandContext(ctx, "docker", "compose", "-f", projectDir+"/docker-compose.yml", action, "-d")
	} else {
		cmd = exec.CommandContext(ctx, "docker", "compose", "-f", projectDir+"/docker-compose.yml", action)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("docker compose %s 失败: %w", action, err)
	}
	return string(out), nil
}

// Stats 获取容器资源使用统计
func (m *Manager) Stats(ctx context.Context, name string) (map[string]interface{}, error) {
	if !m.IsAvailable() {
		return nil, fmt.Errorf("Docker 不可用")
	}
	args := []string{"stats", "--no-stream", "--format", "{{json .}}"}
	if name != "" {
		args = append(args, name)
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker stats 失败: %w", err)
	}

	var result map[string]interface{}
	line := strings.TrimSpace(string(out))
	if line != "" {
		json.Unmarshal([]byte(line), &result)
	}
	return result, nil
}
