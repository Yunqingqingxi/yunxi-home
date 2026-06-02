# 并发与资源管理

> 版本：1.0  
> 适用范围：并发模型、锁策略、资源池、限流、隔离  
> 更新日期：2026-06-02

---

## 1. 概述

Agent 系统天然是并发的：多个用户会话同时进行、每个会话可能派生多个子 Agent、每个子 Agent 可能并发调用多个工具。本文档定义了系统的并发模型、同步策略和资源管理机制。

### 1.1 并发模型全景

```
┌─────────────────────────────────────────────────────┐
│                   HTTP Server                        │
│          (每个请求独立 goroutine)                     │
├─────────────────────────────────────────────────────┤
│  会话 1        会话 2        会话 3        会话 N    │
│  ├─ ReAct Loop  ├─ ReAct Loop  ├─ ReAct Loop        │
│  ├─ 子Agent 1   ├─ 子Agent 3   ├─ 子Agent 5         │
│  ├─ 子Agent 2   ├─ 子Agent 4   ├─ 工具调用...        │
│  ├─ 工具调用...  ├─ 工具调用...                       │
├─────────────────────────────────────────────────────┤
│              Async Executor (Worker Pool)            │
│                    后台任务队列                       │
├─────────────────────────────────────────────────────┤
│       Background Loops (Ticker-based)                │
│  Cleanup(5min) | SyncJob(10min) | Metrics(30s)      │
└─────────────────────────────────────────────────────┘
```

---

## 2. 并发控制模型

### 2.1 并发原语选型

| 原语 | 使用场景 | 示例位置 |
|------|----------|----------|
| `sync.RWMutex` | 读写比高的共享状态 | tracker, session, registry, metrics |
| `sync.Mutex` | 写为主的临界区 | circuit breaker state |
| `atomic.Int64` | 高频计数器 | metrics counters |
| `chan struct{}` | 信号量（并发度控制） | agent manager sem |
| `sync.WaitGroup` | 等待一组 goroutine 完成 | SpawnParallel, shutdown |
| `sync.Map` | 高并发读的缓存 | prompt cache |
| `context.Context` | 取消传播 + 超时控制 | 全系统 |

### 2.2 Channel 信号量模式

系统使用 buffered channel 实现并发度控制：

```go
// agent/manager.go — 子 Agent 并发控制
type Manager struct {
    sem chan struct{}  // 容量 = MaxConcurrent (默认 5)
}

func (m *Manager) Spawn(goal string, toolFilter []string, parentID string) *SubAgent {
    // 获取信号量（阻塞直到有空位）
    m.sem <- struct{}{}
    
    agent := &SubAgent{...}
    go func() {
        defer func() { <-m.sem }()  // 释放信号量
        m.runAgent(agent, parentID)
    }()
    return agent
}
```

**优势**：Go 原生，零依赖，语义清晰  
**注意**：Spawn 是阻塞调用——调用者可能在 `m.sem <- struct{}{}` 处等待

### 2.3 RWMutex 保护模式

```go
// 典型读多写少场景的 RWMutex 使用
type Tracker struct {
    states map[string]*sessionTracker
    mu     sync.RWMutex
}

// 读操作：RLock
func (t *Tracker) GetSession(sessionID string) *sessionTracker {
    t.mu.RLock()
    defer t.mu.RUnlock()
    return t.states[sessionID]
}

// 写操作：Lock
func (t *Tracker) InitSession(sessionID string, constraint Constraint) *sessionTracker {
    t.mu.Lock()
    defer t.mu.Unlock()
    // ...
    t.states[sessionID] = st
    return st
}

// 注意：返回指针后，调用者可能在不持锁的情况下修改
// sessionTracker 的字段。ValidateStep 通过在内部获取锁来保护
```

### 2.4 无锁技术

```go
// 高频计数器使用 atomic
type MetricsCollector struct {
    totalRequests atomic.Int64   // 无锁递增
    totalTokensIn atomic.Int64
    // ...
}

// 环形缓冲区使用原子索引
func (mc *MetricsCollector) Append(entry MetricEntry) {
    idx := atomic.AddInt64(&mc.ringIdx, 1) % int64(cap(mc.ring))
    mc.ring[idx] = entry  // 单写入者，无竞态
}
```

---

## 3. 锁层级与死锁预防

### 3.1 当前锁依赖图

```
Service (chat.go) — 7 个独立 mutex
├── mu          保护 service 自身状态
├── sessionMu   保护 sessionManager（间接）
├── trackerMu   保护 topology.Tracker（间接）
├── agentMu     保护 agent.Manager（间接）
├── execMu      保护工具执行器（间接）
├── eventMu     保护事件总线
└── configMu    保护运行时配置

关键规则：绝不嵌套获取同一层级的两个锁
```

### 3.2 锁顺序约定

```
如果必须持有多把锁，按以下固定顺序获取：

1. sessionMu (最外层)
2. trackerMu
3. agentMu
4. execMu
5. eventMu
6. configMu
7. mu

示例：更新 topology 约束后广播 SSE 事件
  func updateAndNotify() {
      trackerMu.Lock()       // 1. 先锁 tracker
      tracker.UpdateConstraint(...)
      trackerMu.Unlock()
      
      eventMu.Lock()         // 2. 再锁事件总线
      eventBus.Publish(event)
      eventMu.Unlock()
  }
  // ✅ 正确顺序
```

### 3.3 锁持有时间控制

```go
// ❌ 反模式：持锁做 IO
func badPattern(sessionID string) {
    mu.Lock()
    defer mu.Unlock()
    st := states[sessionID]
    data, _ := json.Marshal(st)    // 持锁序列化（可能很大）
    db.Save(data)                   // 持锁做 DB IO（慢！）
}

// ✅ 正确：先复制，释放锁，再做 IO
func goodPattern(sessionID string) {
    mu.RLock()
    st := states[sessionID]
    snapshot := st.Clone()         // 浅拷贝关键数据
    mu.RUnlock()
    
    data, _ := json.Marshal(snapshot)
    db.Save(data)                   // 无锁 IO
}
```

### 3.4 死锁检测建议

```go
// 在开发/测试环境启用 goroutine 死锁检测
import _ "github.com/sasha-s/go-deadlock"

// 将 sync.RWMutex 替换为 deadlock.RWMutex
// 自动检测：
//   - 锁顺序不一致
//   - 持锁时间超过阈值（默认 30s）
//   - 同一 goroutine 重复获取同一把锁
```

---

## 4. 并发执行模型

### 4.1 子 Agent 并行派生

```go
// agent/manager.go — SpawnParallel
func (m *Manager) SpawnParallel(tasks []SpawnTask, parentID string) []*Result {
    var wg sync.WaitGroup
    results := make([]*Result, len(tasks))
    
    for i, task := range tasks {
        wg.Add(1)
        go func(idx int, t SpawnTask) {
            defer wg.Done()
            agent := m.Spawn(t.Goal, t.ToolFilter, parentID)
            results[idx] = m.waitForAgent(agent)
        }(i, task)
    }
    
    wg.Wait()
    return results
}
```

**特征**：
- 使用 `sync.WaitGroup` 等待所有子 Agent 完成
- 结果数组预分配，索引安全（每个 goroutine 写不同位置）
- `Spawn` 内部的 `sem <- struct{}{}` 提供并发度限制

### 4.2 工具并行执行

```go
// middleware/chain.go — ExecuteParallel
func (c *Chain) ExecuteParallel(ctx context.Context, calls []ToolCall) []*ToolResult {
    results := make([]*ToolResult, len(calls))
    var wg sync.WaitGroup
    
    for i, call := range calls {
        wg.Add(1)
        go func(idx int, tc ToolCall) {
            defer wg.Done()
            results[idx] = c.Execute(ctx, tc)
        }(i, call)
    }
    
    wg.Wait()
    return results
}
```

**风险**：无 goroutine 上限。如果 `calls` 很大（例如 AI 一次性请求 50 个工具），会瞬间创建 50 个 goroutine 并可能压垮下游。

**改进**：使用 semaphore 限制并行度：

```go
func (c *Chain) ExecuteParallelBounded(ctx context.Context, calls []ToolCall, maxParallel int) []*ToolResult {
    sem := make(chan struct{}, maxParallel)
    results := make([]*ToolResult, len(calls))
    var wg sync.WaitGroup
    
    for i, call := range calls {
        wg.Add(1)
        go func(idx int, tc ToolCall) {
            defer wg.Done()
            sem <- struct{}{}        // 获取槽位
            defer func() { <-sem }()  // 释放槽位
            results[idx] = c.Execute(ctx, tc)
        }(i, call)
    }
    wg.Wait()
    return results
}
```

### 4.3 Async Executor (Worker Pool)

```go
// async/executor.go — 当前实现
type Executor struct {
    queue    chan *Task      // 缓冲 64
    workers  int             // 默认 3
    wg       sync.WaitGroup
}

func (e *Executor) Submit(task *Task) {
    e.queue <- task          // 阻塞提交
}

// 注意：当前 worker goroutine 主要是骨架代码
// 实际任务执行在 Submit 的 goroutine 中
```

---

## 5. 资源管理

### 5.1 资源类型与管理策略

| 资源类型 | 限制机制 | 默认值 | 实现位置 |
|----------|----------|--------|----------|
| 子 Agent 并发数 | Channel 信号量 | 5 | `agent.Manager.sem` |
| 异步任务队列 | Buffered channel | 64 | `async.Executor.queue` |
| SSH 连接池 | 连接复用 | 按需 | `ssh.Pool` |
| DB 连接池 | `database/sql` 内置 | 由驱动决定 | SQLite/MySQL driver |
| Goroutine 上限 | 无硬限制 | — | 需添加 |
| 内存（消息历史） | BudgetManager | token 级别 | `chat.BudgetManager` |
| 事件缓冲区 | Ring buffer | 200 条 | `chat.eventBus` |

### 5.2 内存管理

```go
// BudgetManager 控制消息历史的 token 预算
type BudgetManager struct {
    maxTokens int      // 消息历史最大 token 数
    // ...
}

// 超预算时触发压缩
func (bm *BudgetManager) CompactHistory(messages []Message) []Message {
    // CompactWithSummary: AI 生成摘要 + 保留最近消息
    // 而非简单截断（保留语义连续性）
}
```

### 5.3 事件总线缓冲区管理

```go
// 事件总线使用 ring buffer + non-blocking send
type sessionEventBus struct {
    buffer      []Event       // 固定大小 ring buffer
    subscribers map[string]chan Event
}

func (b *sessionEventBus) Publish(ev Event) {
    // 写入 ring buffer（供 reconnection 回放）
    b.buffer[b.writeIdx % len(b.buffer)] = ev
    b.writeIdx++
    
    // 推送给订阅者（非阻塞）
    for _, ch := range b.subscribers {
        select {
        case ch <- ev:
        default:
            // 慢消费者被丢弃 — 静默
        }
    }
}
```

**问题**：慢消费者丢弃没有计数器，无法追踪丢弃率。  
**改进**：添加 `atomic.Int64` 计数丢弃事件：

```go
var droppedEvents atomic.Int64
select {
case ch <- ev:
default:
    droppedEvents.Add(1)
}
```

---

## 6. 限流策略

### 6.1 当前限流

```go
// web/server.go — HTTP 层全局限流
e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(20)))
// 20 req/s 全局限流（所有端点共享）
```

### 6.2 推荐：分层限流

```
┌─────────────────────────────────────────┐
│  L1: 全局限流                            │
│  20 req/s — 保护服务不被冲垮              │
├─────────────────────────────────────────┤
│  L2: 用户限流                            │
│  5 req/s per user — 防止单用户滥用       │
├─────────────────────────────────────────┤
│  L3: 工具限流                            │
│  LLM API: 10 req/min per session        │
│  run_command: 5 req/min global          │
│  file_write: 20 req/min global          │
├─────────────────────────────────────────┤
│  L4: 资源限流                            │
│  子Agent: 5 并发 per session            │
│  并行工具: 10 并发 per session           │
└─────────────────────────────────────────┘
```

### 6.3 Token Bucket 实现

```go
// 推荐的 per-tool 限流器
type TokenBucket struct {
    rate     float64    // token/秒
    burst    int        // 突发容量
    tokens   float64
    last     time.Time
    mu       sync.Mutex
}

func (tb *TokenBucket) Allow() bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()
    
    now := time.Now()
    elapsed := now.Sub(tb.last).Seconds()
    tb.tokens = math.Min(float64(tb.burst), tb.tokens + elapsed*tb.rate)
    tb.last = now
    
    if tb.tokens >= 1 {
        tb.tokens--
        return true
    }
    return false
}
```

---

## 7. 资源隔离

### 7.1 会话隔离

| 维度 | 隔离方式 | 说明 |
|------|----------|------|
| **内存** | 每个 session 独立 `sessionTracker` | 不同会话的 topology 状态互不干扰 |
| **消息历史** | 按 sessionID 隔离 | `map[sessionID]*SessionState` |
| **事件流** | 按 sessionID 的独立 eventBus | 每个会话独立的 SSE 事件通道 |
| **子 Agent** | 通过 parentID 归属 | 子 Agent 明确归属父会话 |

### 7.2 文件系统隔离

```go
// 文件访问中间件限制工作目录
type FileAccessMiddleware struct {
    allowedPaths []string         // 白名单路径
    permissions  map[string]int   // 路径级 RBAC
    cache        map[string]bool  // 30s TTL 权限缓存
    mu           sync.RWMutex
}
```

### 7.3 推荐：容器级隔离

对于高安全需求场景，推荐使用容器隔离：

```yaml
# 每个子 Agent 运行在独立容器中
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: sub-agent-executor
    resources:
      limits:
        cpu: "500m"
        memory: "256Mi"
    securityContext:
      readOnlyRootFilesystem: true
      allowPrivilegeEscalation: false
```

---

## 8. 优雅关闭

### 8.1 关闭顺序

```
┌─────────────┐
│ 1. 收到信号  │  SIGTERM / SIGINT
└──────┬──────┘
       ▼
┌─────────────┐
│ 2. 停止接收  │  HTTP Server.Shutdown(ctx)
│    新请求    │  拒绝新连接
└──────┬──────┘
       ▼
┌─────────────┐
│ 3. 完成进行中│  等待现有请求完成（with timeout）
│    的请求    │
└──────┬──────┘
       ▼
┌─────────────┐
│ 4. 取消活跃  │  对所有活跃 session 发送取消信号
│    Agent     │
└──────┬──────┘
       ▼
┌─────────────┐
│ 5. 等待子    │  agent.Manager 等待所有子 Agent 结束
│    Agent     │
└──────┬──────┘
       ▼
┌─────────────┐
│ 6. 刷新状态  │  TopologyTracker.Shutdown() 持久化
│    到 DB     │  等待所有 checkpoint goroutine
└──────┬──────┘
       ▼
┌─────────────┐
│ 7. 关闭 DB   │  关闭数据库连接池
└──────┬──────┘
       ▼
┌─────────────┐
│ 8. 退出进程  │  os.Exit(0)
└─────────────┘
```

### 8.2 实现骨架

```go
func (s *Server) Shutdown(ctx context.Context) error {
    // 1. 停止 HTTP
    s.httpServer.SetKeepAlivesEnabled(false)
    s.httpServer.Shutdown(ctx)
    
    // 2. 取消所有 Agent
    s.chatService.CancelAll("server_shutdown")
    
    // 3. 等待子 Agent（含超时）
    done := make(chan struct{})
    go func() {
        s.agentManager.WaitAll()
        close(done)
    }()
    select {
    case <-done:
    case <-time.After(30 * time.Second):
        slog.Warn("shutdown: agents didn't finish in time")
    }
    
    // 4. 刷新状态
    s.topologyTracker.Shutdown(ctx)
    
    // 5. 关闭 DB
    s.db.Close()
    
    return nil
}
```

---

## 9. 测试策略

### 9.1 单元测试

| 测试ID | 场景 | 操作 | 期望结果 |
|--------|------|------|----------|
| UT-CC-1 | 信号量并发限制 | 并发启动 MaxConcurrent+1 个 Agent | 第 N+1 个阻塞 |
| UT-CC-2 | RWMutex 并发读 | 100 goroutine 并发 GetSession | 无竞态，全部成功 |
| UT-CC-3 | WaitGroup 协调 | SpawnParallel(5 tasks) | 全部完成，结果完整 |
| UT-CC-4 | Atomic 计数器 | 并发 Increment | 最终值 = goroutine 数 |
| UT-CC-5 | 优雅关闭 | 关闭时仍有活跃 Agent | Agent 收到 cancel 信号后结束 |

### 9.2 集成测试

| 测试ID | 场景 | 操作 | 期望结果 |
|--------|------|------|----------|
| IT-CC-1 | 多会话并发 | 5 个会话同时进行 | 各会话独立，结果正确 |
| IT-CC-2 | 子 Agent 池耗尽 | MaxConcurrent=2，Spawn 3 个 | 第 3 个等待，前 2 个完成后启动 |
| IT-CC-3 | 事件总线慢消费 | 快速发布 500 事件 → 慢订阅者 | 旧事件被覆盖，无阻塞 |

### 9.3 竞态检测

```bash
# 启用 Go race detector
go test -race ./internal/ai/agent/...
go test -race ./internal/ai/topology/...
go test -race ./internal/ai/session/...

# CI 中应始终启用
GOFLAGS=-race go test ./...
```

---

## 10. 改进建议

### 短期
- [ ] 为 `ExecuteParallel` 添加并发度限制
- [ ] 为事件总线慢消费者添加丢弃计数
- [ ] 文档化 Service 的 7 个 mutex 锁顺序
- [ ] 清理 async executor 空壳 worker 代码

### 中期
- [ ] 引入 `errgroup` 管理相关 goroutine 的错误传播
- [ ] 实现 per-session 和 per-tool 的 Token Bucket 限流
- [ ] 添加 goroutine 泄漏检测到 CI

### 长期
- [ ] 容器级资源隔离（cgroups/容器）
- [ ] 自适应并发控制（根据系统负载动态调整 MaxConcurrent）
- [ ] 分布式限流（跨实例协调）

---

**文档结束**
