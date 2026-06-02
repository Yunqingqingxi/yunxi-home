# 状态持久化与恢复

> 版本：1.0  
> 适用范围：Session 状态、Topology 轨迹、Agent 上下文、配置的存储与恢复  
> 更新日期：2026-06-02

---

## 1. 概述

在多 Agent 系统中，状态持久化是水平扩展、故障恢复和用户体验的基础。本文档定义了系统的存储分层、检查点策略、恢复机制和幂等性保证。

### 1.1 核心目标

- **断点续传**：系统崩溃或重启后，正在执行的 Agent 任务可以从最近检查点恢复
- **水平扩展**：状态外置化，Agent 实例无状态，任意实例可接管任意会话
- **数据不丢**：关键状态转换和用户可见结果在系统故障后不丢失
- **幂等安全**：重试和恢复不会导致副作用重复执行

### 1.2 状态分类

```
┌─────────────────────────────────────────────────────────┐
│                      状态分层                             │
├─────────────┬──────────────┬──────────────┬─────────────┤
│  热状态      │  温状态       │  冷状态       │  归档状态    │
│  (内存)      │  (DB/Redis)   │  (DB/快照)     │  (日志文件)  │
├─────────────┼──────────────┼──────────────┼─────────────┤
│ 当前推理上下文│ Session 记录  │ 历史会话      │ ChatLogger   │
│ 拓扑 Session │ Topology Node │ 已关闭 Session │ 系统日志     │
│ 事件总线缓冲 │ Config 节     │ 快照          │ 指标快照     │
│ Prompt 缓存  │ User 记录     │               │              │
└─────────────┴──────────────┴──────────────┴─────────────┘
```

---

## 2. 存储选型

### 2.1 三层存储架构

| 层级 | 技术 | 延迟 | 持久性 | 用途 |
|------|------|------|--------|------|
| **内存** | Go map + `sync.RWMutex` | ~100ns | 易失 | 当前推理上下文、活跃会话、事件缓冲、Prompt 缓存 |
| **本地数据库** | SQLite (WAL 模式) | ~1ms | 持久 | Session 记录、Topology 节点、配置、用户、消息历史 |
| **远程数据库** | MySQL | ~5ms | 持久+共享 | 多实例共享、分析查询、跨实例恢复 |

### 2.2 当前实现对照

```go
// 内存层 — 热状态
session.Manager.states    map[string]*SessionState    // sync.RWMutex
topology.Tracker.states   map[string]*sessionTracker  // sync.RWMutex
chat.Service.eventBus     map[string][]*bufferedEvent  // ring buffer
register.Registry.tools   map[string]ToolDef          // sync.RWMutex

// 数据库层 — 温/冷状态
database.ChatRepo         // agent_sessions, messages
topology.SQLiteRepo       // agent_sessions, topology_nodes
config 表                 // JSON blob 按 section 存储
```

### 2.3 双驱动同步架构

当前系统采用 SQLite → MySQL 单向同步：

```
SQLite (主)                    MySQL (从)
──────────                     ─────────
写入路径 ─────→ agent_sessions  ──sync──→ agent_sessions
                messages                   messages
                topology_nodes             (按需复制)
                
同步策略：每 10 分钟全量对账 + 增量写入
批次大小：500 条/批
```

**优势**：单机部署时零依赖，多实例时切换 MySQL 即可。

---

## 3. 检查点策略

### 3.1 检查点触发条件

| 子系统 | 触发条件 | 持久化内容 | 实现位置 |
|--------|----------|-----------|----------|
| Session | 每轮 AI 响应后 | 消息历史 JSON、元数据 | `session.Manager.Save()` |
| Topology | 每 10 个 Node 或 5 秒间隔 | Node 记录 + Session 快照 | `topology.Tracker.checkpointMaybe()` |
| 配置 | 用户保存配置时 | 配置节 JSON | `config.SaveSection()` |
| 指标 | 每 30 秒 | 计数器快照 | `metrics.MetricsCollector.saveLoop()` |
| 中断 | 用户暂停/取消时 | 完整上下文快照 | `interrupt.CancelModeSnapshot` |

### 3.2 Topology 检查点详解

```go
// 双重触发 — 任一满足即触发
func (t *Tracker) checkpointMaybe(sessionID string, st *sessionTracker, node *Node) {
    st.checkpointCount++

    if st.checkpointCount >= CheckpointNodeCount ||           // 10 个节点
       time.Since(st.lastCheckpoint) >= CheckpointInterval {   // 5 秒
        go func() {
            t.repo.SaveNode(ctx, sessionID, node)      // 节点级 INSERT
            t.repo.SaveSession(ctx, st.toRecord())     // 会话级 UPSERT
        }()
        st.checkpointCount = 0
        st.lastCheckpoint = time.Now()
    }
}
```

**设计权衡**：
- 火后即忘（fire-and-forget）goroutine：不阻塞校验主路径，但写入失败仅 Warning 日志
- 10 节点间隔：在正常步速下约 30-60 秒持久化一次，崩溃时最多丢失 10 个节点的轨迹
- 5 秒时间间隔：应对低频调用场景（如用户暂停思考）

### 3.3 Session 保存策略

```go
// 每轮对话后调用
func (m *SessionManager) Save(sessionID string) error {
    m.mu.RLock()
    st := m.states[sessionID]
    m.mu.RUnlock()

    // 序列化消息历史为 JSON
    messagesJSON, _ := json.Marshal(st.Messages)

    // UPSERT — 自动处理新建 vs 更新
    return m.repo.SaveSession(ctx, &SessionRecord{
        SessionID:    sessionID,
        MessagesJSON: string(messagesJSON),
        UpdatedAt:    time.Now(),
        // ... 其他字段
    })
}
```

### 3.4 检查点策略对比

| 策略 | 优点 | 缺点 | 适用场景 |
|------|------|------|----------|
| **每次状态转换后** | 零数据丢失 | 高写入压力 | 金融、支付场景 |
| **每 N 次转换后** | 平衡性能与安全 | 少量数据可丢失 | **当前采用** |
| **定时触发** | 可预测 I/O | 高负载时可能来不及 | **当前采用（补充）** |
| **事件驱动** | 按需触发 | 实现复杂 | 关键操作（暂停/取消） |
| **惰性写入** | 最高性能 | 崩溃风险最大 | 纯缓存场景 |

---

## 4. 恢复机制

### 4.1 恢复流程

```
系统启动
    │
    ▼
┌─────────────────────────────┐
│ 1. RecoverActiveSessions()  │  查询 status IN ('active','interrupted')
│    从 DB 加载所有活跃会话     │
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│ 2. 对每个活跃 Session:       │
│    a. LoadNodes(50)         │  恢复最近 50 个 Topology Node
│    b. 重建 sessionTracker   │  还原坐标、信任状态、约束参数
│    c. 注册到内存 map        │
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│ 3. LoadFromSnapshot()       │  恢复指标计数器快照
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│ 4. 恢复 Prompt 缓存         │  DB → PromptStore 内存缓存
└─────────────┬───────────────┘
              │
              ▼
┌─────────────────────────────┐
│ 5. repairIncompleteCalls()  │  修复崩溃时中断的工具调用
│    注入合成错误结果          │  确保消息历史配对完整
└─────────────────────────────┘
```

### 4.2 崩溃修复：工具调用配对恢复

这是系统最关键的恢复机制。LLM API 要求每次 `assistant` 消息中的 `tool_use` 必须有对应的 `user` 消息（含 `tool_result`）。崩溃时如果 AI 已发起工具调用但未返回结果，消息历史会处于损坏状态。

```go
// 恢复逻辑（session/manager.go）
func (m *SessionManager) repairIncompleteToolCalls(sessionID string) {
    // 1. 遍历消息历史
    // 2. 找到未配对的 tool_use（无对应 tool_result）
    // 3. 插入合成 tool_result：
    //    - status: "error"
    //    - content: "系统已崩溃，工具调用中断。请重新尝试。"
    // 4. 更新消息历史
}
```

**幂等性保证**：合成结果的 `tool_use_id` 与原始请求匹配，LLM 可安全重新发起调用。

### 4.3 Topology 恢复细节

```go
func (t *Tracker) RecoverActiveSessions(ctx context.Context) error {
    records, _ := t.repo.LoadActiveSessions(ctx)
    for _, rec := range records {
        nodes, _ := t.repo.LoadNodes(ctx, rec.SessionID, RecoveryLoadNodes) // 最多 50 个
        st := &sessionTracker{
            SessionID:    rec.SessionID,
            CurrentCoord: rec.CurrentCoord,   // 从 DB 恢复最新坐标
            StartCoord:   rec.StartCoord,
            Nodes:        nodes,              // 最近 50 个轨迹点
            Trust:        TrustState{Lies: rec.TrustLies, Locked: rec.TrustLocked},
            // ...
        }
        t.states[rec.SessionID] = st
    }
}
```

**设计决策**：
- 只恢复最近 50 个 Node：全量恢复历史会话会拖慢启动，50 个节点足以重建趋势
- Trust 状态（Lies/Locked）完整保留：信任不可因重启而"原谅"

### 4.4 断点续传支持

| 场景 | 触发 | 恢复行为 |
|------|------|----------|
| 服务重启 | 进程崩溃/升级 | 自动恢复所有 active 状态会话 |
| 用户暂停 | `CancelModeSnapshot` | 序列化完整上下文到 DB，恢复时还原 |
| 用户取消 | `CancelSoft` / `CancelHard` | 清理会话状态，不恢复 |
| 子 Agent 崩溃 | 进程异常退出 | 父 Agent 收到连接错误，进入 `error` 状态 |
| DB 写入失败 | 网络/磁盘故障 | Checkpoint 仅 Warning 日志，系统降级运行 |

---

## 5. 幂等性

### 5.1 幂等性保护层次

```
┌──────────────────────────────────────────┐
│           Layer 3: 业务幂等               │
│  - 工具调用去重（tool_call_id）            │
│  - Topology Node UNIQUE(session_id,round)│
├──────────────────────────────────────────┤
│           Layer 2: 存储幂等               │
│  - DB UPSERT (INSERT ON CONFLICT)        │
│  - Session Save 使用 UPSERT 语义          │
├──────────────────────────────────────────┤
│           Layer 1: 恢复幂等               │
│  - repairIncompleteToolCalls 合成结果     │
│  - 合成结果的 tool_use_id 与原始匹配      │
└──────────────────────────────────────────┘
```

### 5.2 关键实现

**Topology Node 唯一约束**：
```sql
CREATE TABLE topology_nodes (
    -- ...
    UNIQUE(session_id, round)   -- 同一轮次不会重复插入
)
```

**Session UPSERT**：
```sql
INSERT INTO agent_sessions (...) VALUES (...)
ON CONFLICT(id) DO UPDATE SET
    status=excluded.status,
    current_coord=excluded.current_coord,
    -- ...
```

**工具调用恢复幂等**：
```go
// 系统恢复时注入的合成错误结果
{
    "type": "tool_result",
    "tool_use_id": "<原始 tool_use_id>",  // ← 精确匹配
    "content": "系统已崩溃，工具调用中断。请重新尝试。",
    "is_error": true
}
```

### 5.3 潜在风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 火后即忘 goroutine 写入丢失 | 崩溃时最多丢 10 个 Node | 关键节点同步写入（如 ClosedLoop 完成时） |
| Checkpoint 重复写入 | DB 负载增加 | UPSERT 语义，重复写不产生脏数据 |
| 恢复后 LLM 重复执行工具 | 重复副作用（如重复发送请求） | 系统注入错误结果而非空结果，LLM 可判断是否重试 |
| SyncJob 双向不一致 | MySQL 数据过时 | 每 10 分钟全量对账 + SQLite 为单一事实源 |

---

## 6. 测试策略

### 6.1 单元测试

| 测试ID | 场景 | 操作 | 期望结果 |
|--------|------|------|----------|
| UT-PS-1 | Session 保存 | `Save(sessionID)` | DB 中存在对应记录 |
| UT-PS-2 | Session UPSERT | 连续两次 `Save()` | 只有一条记录，updated_at 更新 |
| UT-PS-3 | Node UPSERT | 同一 round 插入两次 | 只有一条记录，第二条覆盖第一条 |
| UT-PS-4 | 工具调用修复 | 消息历史含未配对 tool_use | 合成 tool_result 插入正确位置 |
| UT-PS-5 | 空会话恢复 | 无 active 会话 | 返回空列表，不 panic |

### 6.2 集成测试

| 测试ID | 场景 | 操作 | 期望结果 |
|--------|------|------|----------|
| IT-PS-1 | 正常重启恢复 | 模拟 crash → 重启 → Recover | 所有 active 会话恢复，坐标正确 |
| IT-PS-2 | Checkpoint 间隔 | 快速提交 5 个 Node → 等待 5s | 时间触发持久化 |
| IT-PS-3 | Checkpoint 计数 | 提交 10 个 Node | 计数触发持久化 |
| IT-PS-4 | 崩溃时未配对工具 | crash 时 tool_use 已发送但无结果 | 恢复后注入合成错误结果 |
| IT-PS-5 | 双驱动同步 | SQLite 写入 → 等待 SyncJob | MySQL 数据一致 |

### 6.3 混沌测试

| 场景 | 注入方式 | 期望行为 |
|------|----------|----------|
| DB 写入超时 | 模拟 DB 连接断开 | Checkpoint 失败仅 Warning，主流程不中断 |
| 内存耗尽 | OOM 杀掉进程 | 重启后恢复最近一次 Checkpoint 状态 |
| 并发写入冲突 | 两个 goroutine 同时 SaveNode | UNIQUE 约束兜底，无 panic |
| 消息历史损坏 | JSON 反序列化失败 | 记录错误，跳过该会话恢复 |
| 恢复时 DB 不可用 | DB 连接被拒绝 | Recover 返回 error，系统等待重试 |

---

## 7. 改进建议

### 7.1 短期（当前迭代）

- [ ] **Checkpoint goroutine 加入 WaitGroup**：`Shutdown()` 时等待所有进行中的 DB 写入完成
- [ ] **恢复时过滤过期会话**：跳过 24 小时无活动的 active 会话
- [ ] **同步写入关键状态变更**：ClosedLoop 完成、信任锁定、连续拒绝等关键事件同步写入 DB

### 7.2 中期（下个版本）

- [ ] **WAL 预写日志**：内存状态变更先写入本地 WAL，异步刷入 DB
- [ ] **快照序列化**：实现快照的版本控制，支持回滚到指定快照
- [ ] **S3/MinIO 快照存储**：大上下文（>10MB）的快照卸载到对象存储
- [ ] **幂等键生成标准化**：所有工具调用生成全局唯一的幂等键

### 7.3 长期（架构演进）

- [ ] **Redis 热状态层**：引入 Redis 作为共享热状态存储，支持真正的多实例无状态
- [ ] **事件溯源**：状态变更作为不可变事件流存储，支持时间旅行调试
- [ ] **双向同步**：MySQL 作为备选事实源，支持从 MySQL 恢复

---

## 8. 附录：存储 Schema 速查

### agent_sessions
```
id, user_id, status, start_coord(JSON), current_coord(JSON),
constraint_json, trust_lies, trust_locked, reject_count,
force_tools_triggered, closed_loop, closed_distance,
created_at, updated_at, last_active_at
```

### topology_nodes
```
id, session_id, round, x, y, z,
tool_call, status, reason, timestamp
UNIQUE(session_id, round)
```

### messages (chat 历史)
```
通过 ChatRepo 管理，消息历史序列化为 JSON 数组存储在列中
```

### config
```
section(VARCHAR), value(JSON), updated_at
```

---

**文档结束**
