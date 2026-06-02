# 配置与扩展

> 版本：1.0  
> 适用范围：配置管理、工具注册、技能系统、MCP 集成、插件机制  
> 更新日期：2026-06-02

---

## 1. 概述

Agent 系统需要高度的可配置性和可扩展性：不同部署环境有不同的参数需求，不同业务场景需要不同的工具集，AI 模型和行为策略需要持续调优。本文档定义了系统的配置管理体系和扩展机制。

### 1.1 核心能力

```
┌────────────────────────────────────────────────────┐
│                   配置与扩展体系                      │
├──────────────┬──────────────┬──────────────────────┤
│  配置管理     │  工具扩展     │  能力扩展             │
├──────────────┼──────────────┼──────────────────────┤
│ YAML 引导     │ 内置工具      │ Skills 技能          │
│ DB 存储       │ 注册表        │ (YAML + Go 编程式)   │
│ 热重载        │ 风险分级      │                      │
│ 分段管理      │ 超时/重试     │ MCP 子系统           │
│ 校验         │              │ (外部工具协议)        │
└──────────────┴──────────────┴──────────────────────┘
```

---

## 2. 配置管理

### 2.1 配置分层

```
加载优先级（后者覆盖前者）：
  1. config.yaml（引导文件）
  2. 环境变量
  3. DB config 表（运行时覆盖）

┌─────────────┐
│ config.yaml │  ← 初始引导 + 默认值
├─────────────┤
│ 环境变量     │  ← CI/容器注入（如 JWT_SECRET）
├─────────────┤
│ DB config   │  ← 运行时通过 API/UI 修改
└─────────────┘
```

### 2.2 Config 结构体

```go
// internal/models/config.go — 17 个子配置段
type Config struct {
    Server    ServerConfig      // 端口、超时、限流
    Database  DatabaseConfig    // SQLite/MySQL 连接
    AI        AIConfig          // LLM Provider、模型、参数
    Auth      AuthConfig        // JWT 密钥、过期时间
    Logger    LoggerConfig      // 日志级别、格式、目录
    DNS       DNSConfig         // DNS 更新配置
    Notification NotificationConfig
    MCP       MCPConfig         // MCP 服务器配置
    Skills    SkillsConfig      // 技能目录、热加载
    Budget    BudgetConfig      // Token 预算策略
    Topology  TopologyConfig    // 拓扑约束默认值
    Chat      ChatConfig        // 会话参数
    Market    MarketConfig      // 市场/工具商店
    // ...
}
```

### 2.3 配置加载流程

```go
// config/loader.go
func LoadFromDB(db *sql.DB, configPath string) (*Config, error) {
    // 1. 读取 config.yaml 作为默认值
    cfg := DefaultConfig()
    if data, err := os.ReadFile(configPath); err == nil {
        yaml.Unmarshal(data, cfg)
    }
    
    // 2. 环境变量覆盖（如 DATABASE_URL, JWT_SECRET）
    applyEnvOverrides(cfg)
    
    // 3. DB 覆盖（通过 config 表）
    rows, _ := db.Query("SELECT section, value FROM config")
    for rows.Next() {
        var section string
        var valueJSON []byte
        rows.Scan(&section, &valueJSON)
        cfg.applySection(section, valueJSON)  // JSON merge
    }
    
    // 4. 校验
    if err := Validate(cfg); err != nil {
        return nil, fmt.Errorf("config validation failed: %w", err)
    }
    
    return cfg, nil
}
```

### 2.4 配置段管理

每个配置段作为 JSON blob 独立存储，支持独立读写：

```
config 表：
┌──────────────┬─────────────┬────────────┐
│ section      │ value       │ updated_at │
├──────────────┼─────────────┼────────────┤
│ ai           │ {"model":... │ ...       │
│ server       │ {"port":...} │ ...       │
│ dns          │ {...}       │ ...       │
│ notification │ {...}       │ ...       │
│ ...          │ ...         │ ...       │
└──────────────┴─────────────┴────────────┘
```

```go
// 配置段 API
func SaveSection(db *sql.DB, section string, value any) error {
    jsonBytes, _ := json.Marshal(value)
    _, err := db.Exec(
        "INSERT INTO config (section, value, updated_at) VALUES (?, ?, ?) "+
        "ON CONFLICT(section) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at",
        section, string(jsonBytes), time.Now(),
    )
    return err
}

func LoadSection(db *sql.DB, section string, target any) error {
    var valueJSON string
    err := db.QueryRow("SELECT value FROM config WHERE section=?", section).Scan(&valueJSON)
    if err != nil {
        return err
    }
    return json.Unmarshal([]byte(valueJSON), target)
}
```

### 2.5 热重载

```go
// 当前支持热重载的配置段
type HotReloadableConfig struct {
    AI       *AIConfig    // 模型切换、temperature 等 → OnAIChanged 回调
    Prompts  *PromptStore // Prompt 模板更新 → 内存缓存失效
    Topology *Constraint  // 拓扑约束参数 → 实时生效
}

// 热重载接口
type ConfigWatcher interface {
    OnChanged(section string, oldValue, newValue any)
}

// 示例：AI 配置变更时通知所有活跃会话
func (s *Service) OnAIChanged(old, new AIConfig) {
    if old.Model != new.Model || old.Provider != new.Provider {
        slog.Info("AI config changed", "model", new.Model, "provider", new.Provider)
        // 通知活跃会话：下一轮对话使用新配置
        s.broadcastConfigChange("ai", new)
    }
}
```

### 2.6 配置校验

```go
// config/validator.go
func Validate(cfg *Config) error {
    var errs []error
    
    // 端口范围
    if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
        errs = append(errs, fmt.Errorf("invalid port: %d", cfg.Server.Port))
    }
    
    // JWT 密钥强度
    if len(cfg.Auth.JWTSecret) < 16 {
        slog.Warn("JWT secret is short, auto-generating")
        cfg.Auth.JWTSecret = generateRandomHex(32)
    }
    
    // AI 参数范围
    if cfg.AI.Temperature < 0 || cfg.AI.Temperature > 2 {
        errs = append(errs, fmt.Errorf("temperature out of range: %f", cfg.AI.Temperature))
    }
    
    // 拓扑参数范围
    if cfg.Topology.A < 0.1 || cfg.Topology.A > 1.0 {
        errs = append(errs, fmt.Errorf("topology A out of [0.1, 1.0]: %f", cfg.Topology.A))
    }
    
    return errors.Join(errs...)
}
```

---

## 3. 工具注册体系

### 3.1 统一工具注册表

```go
// internal/ai/register/registry.go
type Registry struct {
    tools map[string]ToolDef
    mu    sync.RWMutex
}

// 工具定义
type ToolDef struct {
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Parameters  map[string]any    `json:"parameters"`   // JSON Schema
    Handler     ToolHandler       `json:"-"`            // 核心执行函数
    HandlerV2   ToolHandlerV2     `json:"-"`            // 带 context 版本
    RetryPolicy RetryPolicy       `json:"retry_policy"`
    Timeout     time.Duration     `json:"timeout"`
    RiskLevel   RiskLevel         `json:"risk_level"`   // readonly|mutation|dangerous
    Examples    []ToolExample     `json:"examples"`      // Few-shot 示例
    DependsOn   []string          `json:"depends_on"`    // Plan-mode 依赖
}

// 核心操作
func (r *Registry) Register(def ToolDef) error       // 注册工具
func (r *Registry) Get(name string) (ToolDef, bool)    // 按名查询
func (r *Registry) All() []ToolDef                     // 全部工具
func (r *Registry) ByRiskLevel(level RiskLevel) []ToolDef  // 按风险查询
```

### 3.2 工具来源

```
统一工具注册表
├── 内置工具（40+）
│   ├── 文件操作: file_read, file_write, file_edit, file_delete, file_list...
│   ├── 命令执行: run_command
│   ├── DNS 管理: dns_*
│   └── 系统管理: system_info, docker_*, cron_*...
│
├── 技能工具（动态）
│   ├── run_skill(skill_name, params)
│   └── list_skills()
│
└── MCP 工具（外部）
    ├── mcp_filesystem_read_file
    ├── mcp_filesystem_write_file
    ├── mcp_puppeteer_navigate
    └── ...（按 mcp.json 配置动态生成）
```

### 3.3 工具注册流程

```
启动时注册：
  1. toolreg.RegisterAll(registry)      → 内置工具 + ops 工具
  2. skill.RegisterAll(registry)        → 技能包装器工具
  3. mcp.Load() + RegisterTools(reg)    → MCP 外部工具

运行时注册：
  POST /api/market/install-mcp          → 安装新 MCP 服务器
    → 增量注册新工具到 Registry
    → 不中断已有工具
```

### 3.4 工具命名规范

```
内置工具:    <category>_<action>        例: file_read, dns_update
技能工具:    run_skill / list_skills
MCP 工具:    mcp__<server>__<tool>      例: mcp__filesystem__read_file

命名约定：
  - 全小写 + 下划线
  - 动词在后（file_read 而非 read_file —— 与现有代码对齐）
  - 通配符匹配（get_*, docker_* 用于风险画像）
```

---

## 4. Skills 技能系统

### 4.1 双轨制模型

```
Skills 技能层
├── YAML Manifest（声明式）
│   ├── skills/<name>/SKILL.md
│   ├── 简单流程编排
│   └── 非开发人员可编写
│
└── Programmatic Go（编程式）
    ├── 实现 ProgrammaticSkill 接口
    ├── 复杂逻辑、类型安全、高性能
    └── 编译时注册
```

### 4.2 核心接口

```go
type ProgrammaticSkill interface {
    Name() string
    Description() string
    Category() string          // ops | file | dns | system | general
    RiskLevel() string         // readonly | mutation | dangerous
    Run(ctx context.Context, params map[string]any, mcp MCPContext) (any, error)
}

// 可选扩展接口
type SkillWithSchema interface {
    ProgrammaticSkill
    Schema() json.RawMessage   // JSON Schema 参数校验
}

type SkillWithExamples interface {
    ProgrammaticSkill
    Examples() []ToolExample   // AI few-shot 调用示例
}
```

### 4.3 技能注册与执行

```
Agent 调用 run_skill(skill_name, params)
  │
  ▼
Executor.Run(name, params)
  │
  ├─ 1. 查找 Registry（编程式优先，最快）
  ├─ 2. 回退 YAML Loader（从 skills/ 目录加载）
  ├─ 3. 参数 Schema 校验
  ├─ 4. 执行（通过 MCPContext 安全访问 HTTP/日志/缓存）
  └─ 5. 返回结果
```

### 4.4 YAML 技能示例

```yaml
# skills/system_health/SKILL.md
---
name: system_health
description: 检查系统健康状态（CPU、内存、磁盘、服务）
category: ops
risk_level: readonly
parameters:
  type: object
  properties:
    include_services:
      type: boolean
      description: 是否包含服务状态检查
      default: false
steps:
  - tool: run_command
    args:
      command: "top -bn1 | head -5"
    description: 获取 CPU 和内存使用
  - tool: run_command
    args:
      command: "df -h"
    description: 检查磁盘使用
  - condition: "{{.include_services}}"
    then:
      - tool: run_command
        args:
          command: "systemctl status nginx docker --no-pager"
        description: 检查关键服务
```

---

## 5. MCP 子系统

### 5.1 架构

```
MCP Subsystem
  │
  ├── Manager（连接生命周期）
  │   ├── 读取 mcp.json 配置
  │   ├── 启动 MCP 服务器进程（stdio）
  │   ├── 管理连接池
  │   ├── 健康检查 + 自动重连
  │   └── 优雅关闭
  │
  ├── Registry Adapter
  │   ├── 将 MCP 工具以 mcp_<server>_<tool> 命名
  │   ├── 参数 Schema 转换
  │   └── 注册到统一 Registry
  │
  ├── InstallTracker
  │   ├── 跟踪安装进度
  │   └── SSE 实时反馈
  │
  └── Config（mcp.json）
      └── 增量安装，不中断已有连接
```

### 5.2 MCP 配置

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@anthropic/mcp-server-filesystem", "/workspace"],
      "env": {}
    },
    "puppeteer": {
      "command": "npx",
      "args": ["-y", "@anthropic/mcp-server-puppeteer"],
      "env": {"PUPPETEER_HEADLESS": "true"}
    }
  }
}
```

### 5.3 工具发现与调用

```
启动时：
  Subsystem.Load() → 读取 mcp.json
  Manager.ConnectAll() → 启动所有 MCP 服务器
  RegisterTools() → 将每个 MCP 工具注入统一注册表

运行时：
  Agent 调用 mcp_filesystem_read_file
    → Registry.Get("mcp_filesystem_read_file")
    → MCP Adapter.CallTool(ctx, "filesystem", "read_file", args)
    → MCP Client 发送 JSON-RPC 到 stdio
    → 返回结果给 Agent

热加载：
  POST /api/market/install-mcp → 安装新 MCP 服务器
    → 增量注册工具（不重启已有连接）
    → InstallTracker 推送进度到 SSE
```

---

## 6. Prompt 管理

### 6.1 三层提示词体系

```
提示词来源（优先级从低到高）：
  1. Go 常量（代码内置）         — 不可变，作为兜底
  2. DB prompt_sections 表       — 管理员通过 API 修改
  3. 内存 PromptStore 缓存       — 运行时高速访问
```

### 6.2 Prompt 热重载

```go
// PromptStore 支持运行时热更新
type PromptStore struct {
    sections map[string]string      // 按 section 存储的 Prompt 片段
    cache    sync.Map               // 跨会话的 intent hash 缓存
    mu       sync.RWMutex
}

// API 端点
PUT  /api/admin/prompts/:section    → UpdatePromptSection(section, content)
POST /api/admin/prompts/:section/reset → ResetPromptSection(section) // 恢复默认

// 热重载流程
func (ps *PromptStore) UpdateSection(section, content string) error {
    ps.mu.Lock()
    ps.sections[section] = content
    ps.mu.Unlock()
    
    ps.SaveToDB(section, content)    // 持久化
    ps.InvalidateCache(section)      // 失效相关缓存
    
    slog.Info("Prompt section updated", "section", section)
    return nil
}
```

---

## 7. 动态扩展机制

### 7.1 当前支持的扩展方式

| 扩展类型 | 方式 | 热加载 | 示例 |
|----------|------|--------|------|
| **内置工具** | Go 编译时注册 | ❌ 需重启 | `toolreg.RegisterAll()` |
| **MCP 工具** | JSON 配置 + npm 安装 | ✅ 增量 | `mcp.json` + `POST /api/market/install-mcp` |
| **YAML 技能** | skills/ 目录文件 | ✅ 文件变更检测 | `skills/backup/SKILL.md` |
| **Go 编程技能** | 实现接口 + 编译 | ❌ 需重编译 | `skill.Register("my_skill", impl)` |
| **Prompt** | API + DB | ✅ 即时 | `PUT /api/admin/prompts/system` |
| **风险画像** | API 注册 | ✅ 即时 | `RegisterRiskProfile(profile)` |
| **配置段** | API + DB | ✅ 即时（部分） | `PUT /api/admin/config/ai` |

### 7.2 推荐：插件系统架构

```go
// 中长期演进：Go 插件系统（基于 hashicorp/go-plugin）
type Plugin interface {
    Name() string
    Version() string
    Tools() []ToolDef         // 插件提供的工具
    Skills() []ProgrammaticSkill  // 插件提供的技能
    Init(ctx context.Context, cfg PluginConfig) error
    Shutdown(ctx context.Context) error
}

// 插件管理器
type PluginManager struct {
    plugins map[string]Plugin
}

// 插件生命周期
func (pm *PluginManager) Load(path string) error {
    // 1. 从 .so 文件加载（go plugin）或通过 stdio 协议
    // 2. 调用 Plugin.Init()
    // 3. 注册 Tools + Skills 到全局注册表
    // 4. 监听健康状态
}
```

---

## 8. A/B 测试与灰度

### 8.1 推荐的灰度框架

```go
// 基于用户/会话的特征路由
type ExperimentConfig struct {
    Name       string            `json:"name"`
    Variants   []Variant         `json:"variants"`
    Traffic    float64           `json:"traffic"`     // 0.0 - 1.0
    Targeting  TargetingRule     `json:"targeting"`   // 用户特征匹配
}

type Variant struct {
    Name   string         `json:"name"`
    Weight float64        `json:"weight"`
    Config map[string]any `json:"config"`   // 覆盖的配置项
}

// 示例：模型 A/B 测试
experiment := ExperimentConfig{
    Name: "model-v4-vs-v5",
    Variants: []Variant{
        {Name: "control",  Weight: 0.5, Config: map[string]any{"model": "claude-sonnet-4-6"}},
        {Name: "treatment", Weight: 0.5, Config: map[string]any{"model": "claude-opus-4-8"}},
    },
    Traffic: 0.2,  // 20% 的用户参与实验
}
```

---

## 9. 测试策略

| 测试ID | 场景 | 操作 | 期望结果 |
|--------|------|------|----------|
| UT-CE-1 | 配置加载优先级 | YAML + ENV + DB 三层都有值 | DB 值生效 |
| UT-CE-2 | 配置校验失败 | port=99999 | 返回 error |
| UT-CE-3 | 工具重复注册 | 同名工具注册两次 | 第二次返回 error 或覆盖 |
| UT-CE-4 | Prompt 热重载 | 运行时 update → 重新获取 | 新 Prompt 生效 |
| UT-CE-5 | MCP 工具注册 | mcp.json 含 2 个服务器 | 所有工具注册成功 |
| UT-CE-6 | 风险画像动态注册 | RegisterRiskProfile(new) | MatchRiskProfile 命中新规则 |
| IT-CE-1 | MCP 增量安装 | 运行中安装新 MCP → 调用新工具 | 新工具可用，旧工具不受影响 |
| IT-CE-2 | 配置变更通知 | 修改 AI model → 新会话使用新模型 | 活跃会话在下一轮切换 |

---

**文档结束**
