package register

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/yxd/yunxi-home/internal/ai/base"
)

// Registry 工具注册中心
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*base.ToolDef
}

// New 创建工具注册中心
func New() *Registry {
	return &Registry{tools: make(map[string]*base.ToolDef)}
}

// Register 注册一个工具
func (r *Registry) Register(t *base.ToolDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name] = t
}

// Get 获取工具定义
func (r *Registry) Get(name string) (*base.ToolDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// All 获取所有工具定义
func (r *Registry) All() []base.ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]base.ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, *t)
	}
	return result
}

// Execute 执行指定工具
func (r *Registry) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	r.mu.RLock()
	t, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("未知工具: %s", name)
	}
	return t.Handler(ctx, args)
}

// ParseArgs 解析 JSON 参数
func ParseArgs(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, fmt.Errorf("解析参数失败: %w", err)
	}
	return args, nil
}
