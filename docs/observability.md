# 可观测性

> 版本：1.0  
> 适用范围：日志、指标、追踪、告警的完整可观测性体系  
> 更新日期：2026-06-02

---

## 1. 概述

可观测性是多 Agent 系统运维和调试的基石。Agent 系统的特殊性——长链路、非确定性推理、嵌套调用——要求比传统应用更丰富的观测手段。本文档定义了系统的四大观测支柱：**日志、指标、追踪、告警**。

### 1.1 设计原则

- **结构化优先**：所有日志和事件使用结构化格式（JSON），支持机器解析
- **Trace ID 贯穿**：每个用户请求生成唯一 trace_id，贯穿整个调用链
- **状态转换可见**：Agent 状态机每次转换都有日志埋点
- **分级告警**：区分信息、警告、严重三层，避免告警疲劳

---

## 2. 日志体系

### 2.1 架构概览

```
┌──────────────────────────────────────────────────────┐
│                    应用程序                            │
│  slog.Debug() / slog.Info() / slog.Warn() / slog.Error()
└────────┬──────────┬──────────┬──────────┬───────────┘
         │          │          │          │
         ▼          ▼          ▼          ▼
┌────────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐
│ 系统日志    │ │ 聊天追踪  │ │ 指标日志  │ │ 错误日志      │
│ yunxi-     │ │ log/chat/ │ │ metrics   │ │ 分布在        │
│ home.log   │ │           │ │ snapshot  │ │ 各模块中      │
└────────────┘ └──────────┘ └──────────┘ └──────────────┘
```

### 2.2 系统日志

**技术栈**：Go 标准库 `log/slog`，支持 text/JSON 双格式

```go
// logger/logger.go — 日志初始化
func InitLogger(logDir string, level slog.Level, format string) {
    // 1. 多输出：stdout + 文件（io.MultiWriter）
    // 2. 文件轮转：50MB 自动轮转，保留 5 个备份
    // 3. 目录结构：log/YYYY/MM/DD/yunxi-home.log
    // 4. 时区转换：ReplaceAttr 统一为 CST(UTC+8)
    // 5. 重复合并：DedupWriter 压缩连续相同行 → "[repeated Nx]"
    // 6. 运行时级别切换：levelGuardHandler 支持 HTTP API 动态调整
}
```

#### 日志级别使用规范

| 级别 | 使用场景 | 示例 |
|------|----------|------|
| `Debug` | 开发调试、详细的状态机追踪 | `"坐标校验通过"`, `"Prompt 缓存命中"` |
| `Info` | 正常业务流程、状态转换 | `"会话已创建"`, `"约束参数已更新"`, `"拓扑会话恢复完成"` |
| `Warn` | 可恢复异常、性能劣化、接近阈值 | `"保存拓扑节点失败"`, `"信任已锁定"`, `"连续拒绝 5 次"` |
| `Error` | 不可恢复错误、数据损坏、系统失败 | `"LLM API 调用失败"`, `"恢复会话失败"` |

#### 关键埋点位置

```
internal/ai/chat.go:          每轮 ReAct 循环的入口/出口
internal/ai/topology/tracker.go: ValidateStep 校验结果
internal/ai/topology/tracker.go: 信任状态变更（Lies 递增、Locked 触发）
internal/ai/agent/manager.go:    子 Agent 创建/完成/超时
internal/ai/session/manager.go:  Session 创建/保存/销毁
internal/ai/mcp/subsystem.go:    MCP 服务器连接/重连/健康检查
internal/database/syncer.go:     双驱动同步状态
```

### 2.3 聊天追踪日志

专为 AI 对话场景设计的结构化追踪，输出到 `log/chat/` 目录：

```go
// ChatLogger 记录每次对话的完整轨迹
type ChatLogEntry struct {
    SessionID  string    `json:"session_id"`
    TraceID    string    `json:"trace_id"`
    Round      int       `json:"round"`
    Event      string    `json:"event"`       // start, end, tool_call, error
    Tools      []string  `json:"tools"`
    Duration   int64     `json:"duration_ms"`
    TokensIn   int       `json:"tokens_in"`
    TokensOut  int       `json:"tokens_out"`
    Error      string    `json:"error,omitempty"`
    Timestamp  time.Time `json:"timestamp"`
}
```

**用途**：离线分析对话质量、工具使用模式、LLM 延迟趋势。

### 2.4 日志格式规范

**结构化字段约定**：

```json
{
  "time": "2026-06-02T15:04:05.000+0800",
  "level": "INFO",
  "msg": "信任已锁定",
  "session": "sess_abc123",        // 会话 ID
  "trace_id": "tr_def456",         // 请求追踪 ID（如有）
  "agent": "main",                 // 组件名
  "round": 12,                     // 对话轮次（AI 场景）
  "duration_ms": 2340,             // 耗时（性能敏感操作）
  "error": "connection refused"    // 错误详情（ERROR 级别）
}
```

**反模式**（应避免）：
- ❌ 自由文本日志：`"处理完成"`
- ✅ 结构化日志：`"msg":"拓扑节点已提交", "session":"sess_123", "round":5, "status":"committed"`

---

## 3. 指标体系

### 3.1 架构

```
MetricsCollector (内存)
├── 原子计数器（atomic.Int64）
│   ├── total_requests        请求总数
│   ├── total_errors          错误总数
│   ├── total_tokens_in       输入 Token 总数
│   ├── total_tokens_out      输出 Token 总数
│   └── total_cost            总费用估算
├── 工具延迟直方图（sync.RWMutex）
│   ├── file_read: P50/P95/P99
│   ├── run_command: P50/P95/P99
│   └── ...
└── 环形缓冲区（16384 条）
    └── 最近事件快照

持久化：
├── 计数器快照 → DB 每 30 秒
└── 启动恢复 → LoadFromSnapshot()
```

### 3.2 核心指标定义

#### 业务指标

| 指标名 | 类型 | 单位 | 说明 |
|--------|------|------|------|
| `agent_requests_total` | Counter | 次 | 总请求数 |
| `agent_errors_total` | Counter | 次 | 总错误数 |
| `agent_tokens_in_total` | Counter | token | 输入 Token 总量 |
| `agent_tokens_out_total` | Counter | token | 输出 Token 总量 |
| `agent_cost_total` | Counter | USD | 估算费用 |
| `agent_active_sessions` | Gauge | 个 | 当前活跃会话数 |

#### 性能指标

| 指标名 | 类型 | 单位 | 说明 |
|--------|------|------|------|
| `agent_round_duration_ms` | Histogram | ms | 单轮对话耗时 |
| `agent_tool_duration_ms` | Histogram | ms | 工具调用耗时（按工具名分片） |
| `agent_llm_latency_ms` | Histogram | ms | LLM API 响应延迟 |
| `agent_session_duration_s` | Histogram | s | 完整会话耗时 |

#### 质量指标

| 指标名 | 类型 | 单位 | 说明 |
|--------|------|------|------|
| `agent_topology_reject_total` | Counter | 次 | 拓扑校验拒绝次数 |
| `agent_topology_lies_total` | Counter | 次 | AI 坐标谎报次数 |
| `agent_trust_locked` | Gauge | 0/1 | 信任是否锁定（按会话） |
| `agent_subagent_success_rate` | Gauge | % | 子 Agent 成功率 |
| `agent_oscillation_detected` | Counter | 次 | 工具震荡检测次数 |
| `agent_closed_loop_success` | Counter | 次 | 闭环成功次数 |

#### 系统指标

| 指标名 | 类型 | 单位 | 说明 |
|--------|------|------|------|
| `agent_event_buffer_size` | Gauge | 条 | 事件缓冲区当前大小 |
| `agent_goroutine_count` | Gauge | 个 | 活跃 goroutine 数 |
| `agent_db_checkpoint_latency_ms` | Histogram | ms | DB 检查点写入延迟 |
| `agent_sync_job_duration_s` | Histogram | s | SQLite→MySQL 同步耗时 |

### 3.3 工具延迟分位统计

```go
// 按工具名分组，记录 P50/P95/P99
type ToolStats struct {
    ToolName string
    Count    int64
    P50      time.Duration
    P95      time.Duration
    P99      time.Duration
}

// 每次工具调用完成时更新
func (mc *MetricsCollector) RecordToolCall(toolName string, duration time.Duration) {
    mc.mu.Lock()
    defer mc.mu.Unlock()
    stats := mc.toolStats[toolName]
    // 更新分位统计
}
```

### 3.4 指标导出

**当前状态**：指标仅在内存中收集，通过 HTTP API 查询快照。

**推荐改进**（中期）：

```go
// 暴露 Prometheus /metrics 端点
import "github.com/prometheus/client_golang/prometheus"

var (
    agentRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "agent_requests_total"},
        []string{"session_id", "status"},
    )
    agentToolDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "agent_tool_duration_seconds",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
        },
        []string{"tool_name"},
    )
)
```

---

## 4. 分布式追踪

### 4.1 追踪模型

```
外部请求
  │  trace_id: "tr_abc123"
  ▼
┌─────────────────────────┐
│  主 Agent (span_1)       │  service: "agent.main"
│  ├─ LLM 推理 (span_1.1)  │  duration: 2.3s
│  ├─ 工具调用 (span_1.2)   │  duration: 0.5s
│  └─ 子 Agent 委派 (span_1.3)
│       │
│       ▼
│  ┌─────────────────────┐
│  │ 子 Agent (span_2)    │  service: "agent.sub"
│  │ ├─ 工具调用 (span_2.1)│  parent: span_1.3
│  │ └─ 工具调用 (span_2.2)│
│  └─────────────────────┘
└─────────────────────────┘
```

### 4.2 当前实现与改进方向

**当前状态**：系统未集成 OpenTelemetry，无跨组件追踪。

**推荐实现路径**：

```go
// 1. 在 HTTP 入口生成 trace_id
func TraceMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        traceID := c.Request().Header.Get("X-Trace-ID")
        if traceID == "" {
            traceID = generateTraceID()
        }
        ctx := context.WithValue(c.Request().Context(), traceKey{}, traceID)
        c.SetRequest(c.Request().WithContext(ctx))
        c.Response().Header().Set("X-Trace-ID", traceID)
        return next(c)
    }
}

// 2. 在关键路径创建 span
func (s *Service) StreamChat(ctx context.Context, ...) {
    ctx, span := otel.Tracer("agent").Start(ctx, "StreamChat")
    defer span.End()
    span.SetAttributes(
        attribute.String("session_id", sessionID),
        attribute.Int("round", round),
    )
    // ...
}

// 3. 工具调用自动埋点
func (c *Chain) Execute(ctx context.Context, call ToolCall) *ToolResult {
    ctx, span := otel.Tracer("tool").Start(ctx, "tool."+call.Name)
    defer span.End()
    // ...
}
```

### 4.3 追踪上下文传递

| 传递方式 | 场景 | 实现 |
|----------|------|------|
| `context.Context` | 同进程内 | `context.WithValue(ctx, traceKey, traceID)` |
| HTTP Header | 跨服务 | `X-Trace-ID`, `X-Span-ID` |
| gRPC Metadata | 内部 RPC | W3C TraceContext |
| 日志注入 | 结构化日志 | `slog.With("trace_id", traceID)` |

---

## 5. 告警规则

### 5.1 告警分级

| 级别 | 响应时间 | 通知方式 | 示例 |
|------|----------|----------|------|
| **P0 - 紧急** | 5 分钟内 | 电话 + 即时消息 | LLM API 完全不可用、DB 损坏 |
| **P1 - 严重** | 15 分钟内 | 即时消息 + 邮件 | 错误率 >10%、信任锁定额外增长 |
| **P2 - 警告** | 1 小时内 | 邮件 | 连续拒绝 >5 次、工具超时增加 |
| **P3 - 通知** | 24 小时内 | Dashboard | 新工具注册、配置变更 |

### 5.2 关键告警规则

```yaml
# 业务告警
- alert: HighErrorRate
  expr: rate(agent_errors_total[5m]) / rate(agent_requests_total[5m]) > 0.1
  for: 5m
  severity: P1
  description: "Agent 错误率超过 10%"

- alert: TrustLocked
  expr: agent_trust_locked == 1
  for: 1m
  severity: P2
  description: "会话 {{ $labels.session_id }} 信任已锁定"

- alert: ContinuousRejects
  expr: rate(agent_topology_reject_total[5m]) > 5
  for: 5m
  severity: P2
  description: "拓扑连续拒绝超过阈值"

# 性能告警
- alert: HighLLMLatency
  expr: histogram_quantile(0.95, agent_llm_latency_ms) > 10000
  for: 10m
  severity: P1
  description: "LLM P95 延迟超过 10 秒"

- alert: ToolTimeoutSpike
  expr: rate(agent_tool_timeout_total[5m]) > 0.05
  for: 5m
  severity: P2
  description: "工具超时率超过 5%"

# 系统告警
- alert: HighGoroutineCount
  expr: agent_goroutine_count > 1000
  for: 5m
  severity: P2
  description: "Goroutine 泄漏或堆积"

- alert: EventBufferFull
  expr: agent_event_buffer_size > 180  # 200 上限的 90%
  for: 1m
  severity: P2
  description: "事件缓冲区接近饱和"

- alert: DBCheckpointFailure
  expr: rate(agent_db_checkpoint_errors_total[5m]) > 0
  for: 10m
  severity: P1
  description: "DB 检查点写入失败"
```

### 5.3 静默与抑制

- **维护窗口**：计划内升级期间抑制所有 P2/P3 告警
- **依赖级联抑制**：LLM API 不可用时，抑制由此引发的工具超时告警
- **重复抑制**：相同 session_id 的同类告警 10 分钟内不发第二次

---

## 6. Dashboard 设计

### 6.1 运维总览面板

```
┌─────────────────────────────────────────────────────┐
│  Active Sessions: 12    Requests/min: 45    OK: 98% │
├──────────────────┬──────────────────────────────────┤
│  错误率趋势       │  工具调用 Top 5                   │
│  ▁▂▁▃▁▂▁          │  file_read:   234               │
│                  │  run_command:  89               │
│                  │  mcp__*:       56               │
│                  │  file_write:   34               │
│                  │  spawn_agent:  12               │
├──────────────────┼──────────────────────────────────┤
│  LLM 延迟 P95    │  信任状态分布                     │
│  ▂▃▅▃▂▂▃         │  Normal: 10  Suspicious: 1      │
│                  │  Locked: 1                       │
├──────────────────┴──────────────────────────────────┤
│  系统资源: CPU ▃▅▂  Memory ▂▃▅  Goroutines: 234    │
└─────────────────────────────────────────────────────┘
```

### 6.2 会话详情面板

```
Session: sess_abc123
├── 状态: active  轮次: 12/100  信任: Normal(0 lies)
├── Topology: X=5.2 Y=0.3 Z=1.1
├── 工具使用: file_read(4), run_command(3), file_write(2), spawn_agent(2), mcp__puppeteer(1)
├── 校验: committed(10), rejected(1), overridden(1)
├── Token: 输入=12,345  输出=4,567  费用=$0.23
├── 子 Agent: agent_1(done), agent_2(running)
└── 耗时: 平均=2.3s/轮  P95=5.1s  总=27.6s
```

---

## 7. 健康检查

### 7.1 端点定义

```go
// GET /health — 存活检查
// 返回 200 当进程存活
// 不做任何依赖检查

// GET /ready — 就绪检查
// 返回 200 当所有依赖就绪：
//   - DB 连接可用
//   - MCP 子系统加载完成
//   - Prompt Store 初始化完成
// 返回 503 当任一依赖未就绪
```

### 7.2 依赖健康矩阵

| 依赖 | 检查方式 | 超时 | 降级策略 |
|------|----------|------|----------|
| SQLite | `db.Ping()` | 1s | 内存优先，异步写入队列 |
| MySQL | `db.Ping()` | 2s | 确认 SQLite 可用即可就绪 |
| LLM API | 最近一次成功时间 | N/A | 请求进入待处理队列 |
| MCP 服务 | 进程存活 + stdio 连接 | 5s | 标记对应工具为不可用 |

---

## 8. 实施路线图

### Phase 1 — 当前（已实现）
- [x] 结构化日志（slog + JSON）
- [x] 日志轮转（50MB + 5 备份）
- [x] 日志去重（DedupWriter）
- [x] 运行时级别切换
- [x] 聊天追踪日志（ChatLogger）
- [x] 内存指标收集（MetricsCollector）
- [x] 指标快照持久化（30s）
- [x] 基础健康检查（/health, /ready）

### Phase 2 — 短期（1-2 迭代）
- [ ] Trace ID 生成与注入（请求入口中间件）
- [ ] 工具调用自动耗时统计
- [ ] 子 Agent 成功/失败率 Dash
- [ ] 告警规则 Webhook 集成
- [ ] 日志聚合（Loki / ELK）
- [ ] 请求速率 Dashboard

### Phase 3 — 中期（下个大版本）
- [ ] OpenTelemetry 全链路追踪
- [ ] Prometheus metrics 端点
- [ ] Grafana Dashboard 模板
- [ ] 分布式追踪可视化（Jaeger）
- [ ] 异常检测（基于历史基线的自动告警）

### Phase 4 — 长期（架构演进）
- [ ] AI 辅助日志分析（自动诊断常见问题）
- [ ] 对话质量自动评分
- [ ] 成本归因（按会话/用户/工具的成本拆解）
- [ ] 预测性告警（基于趋势的容量预警）

---

**文档结束**
