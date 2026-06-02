# 拓扑约束系统 — 任务强制完成设计

## 核心思想

用 3D 向量距离量化任务完成度，替代碎片化的启发式规则。

## 坐标空间

```
X: 任务进度 0-10  (0=未开始, 10=完成)
Y: 复杂度变化 -1.0~1.0  (负=降复杂度, 正=增复杂度)
Z: 偏离起点距离 0~R  (R=约束半径, 默认3.0)
```

## 任务完成判定

```
目标点: (10, 0, 0)  ← 任务完美完成
当前点: AI 自报或 RiskProfile 估算

dist = √((10-X)² + (0-Y)² + (0-Z)²)

dist < 0.05  →  ✅ 精确完成，放行
dist ≥ 0.05  →  ❌ 未完成，拦截回复，注入重试指令
```

## 为什么优于启发式规则

| 启发式方案 | 量化方案 |
|---|---|
| "有工具失败了" → 拦截 | X 从 2→3，dist 缩小 50% → 不拦截（AI 在进步） |
| "AI 给了纯文本" → 拦截 | AI 说"搜索不到"但 X 没动 → 拦截 |
| 找不到失败工具名就放行 | 只看坐标，不关心哪一步失败 |
| 硬编码 3 次重试 | Y 轴累积超 R → 自动降复杂度，防止死循环 |

## 降级路径

AI 不报 `<topology>` 标签时：
1. 用 `RiskProfile` 对照表估算坐标变化
2. 信任度连续 3 次谎报 → 锁定模式（忽略 AI 自报，只用估算值）
3. 连续拒绝 5 次 → 触发救援（放行一次 / 手动调配）

## 闭环强制 (T=true)

```
X ≥ 9.5 时触发:
  dist = √(X² + Y² + Z²)  ← 距离原点
  dist ≤ 0.5  →  ✅ 闭环达成
  dist > 0.5  →  ❌ "任务未闭合(差距=%.2f)"，强制补充回环操作
```

## ForceTools

```
X ≥ 5.0 且最近 N 轮未使用约束工具:
  → 跳过 LLM，直接执行 ForceTools 列表中的工具
  → 确保关键路径不被跳过
```

## 实现要点

1. `hadRecentToolFailure` 废弃，改为 `dist < 0.05` 检测
2. AI 不报标签时回退 `RiskProfile` 估算
3. checkpoint 写入 DB，跨轮追踪进度趋势
4. 弃疗拦截放在 `finalContent` 输出前

---

# 意图向量化系统 — 两级流水线设计

## 目标

将用户自然语言消息映射到工具调用意图，精度目标 **>90%**（TF 规则层 64%，嵌入模型层预期 90%+）。

## 流水线架构

```
用户消息
  ↓
Stage 1: 精确触发器（手写规则，精度 86%，覆盖率 20%）
  - 命令匹配: "docker ps" → docker_list_containers
  - 工具名: "reload_mcp" → reload_mcp
  - 强关键词: "DNS记录" → query_dns_records
  - 命中 → 直接路由，不走后续阶段
  ↓ 未命中
Stage 2: 嵌入向量匹配（BGE-small-zh，预期精度 90%+）
  - 用户消息 → 512维向量
  - 与所有工具描述向量算余弦相似度
  - 取 Top-K，品类聚合
  ↓
Stage 3: 信息类兜底
  - S2 无强匹配 → 默认路由 web_search
  - 闲聊过滤: max(cos) < 0.5 → 无意图
```

## Benchmark（225 条手工标注数据）

| 指标 | TF + 词典 | BGE 嵌入（预期） |
|---|---|---|
| Top-1 准确率 | 47% | 85-92% |
| 品类匹配率 | 69% | 92-96% |
| 闲聊过滤率 | 100% | 98-100% |
| search 品类 | 17% | 85%+ |
| 复合意图 F1 | ~0% | 60-75% |

## 模型选型

**BGE-small-zh (BAAI)**
- 大小: 24MB（ONNX 导出）
- 维度: 512
- 推理延迟: ~5ms（CPU）
- 许可证: MIT
- 中文优化，HuggingFace MTEB 中文榜单 Top-3

## 工程方案

### 方案 A: Go + ONNX Runtime（推荐）

```
启动 → 加载 onnxruntime.so + bge-small-zh.onnx (24MB)
     → 嵌入所有工具描述 (20 × 512 = 40KB) → 缓存到 config 表
运行时 → 用户消息 → ONNX 推理 (5ms) → 512维向量
     → 与工具向量算余弦 (20 次点积, <0.1ms)
     → 返回 Top-K
重启 → 从 DB 读工具向量缓存（40KB），跳过工具侧推理
```

DB 缓存:
```sql
config 表:
  section: "tool_embedding_file_list"
  data: [0.12, -0.34, ...]  -- JSON float64 数组
```

### 方案 B: Python HTTP 微服务

```
Go → POST /embed {"texts": [...]} → Python sentence-transformers → [vectors]
```

- 优点: 模型更新不需要重启 Go
- 缺点: 网络延迟 +5ms，多一个进程要维护

### 方案 C: DeepSeek API

- 优点: 零本地资源
- 缺点: 每次调用 ~200ms，网络依赖，有成本

## Stage 1 触发器设计

### 规则引擎

```
每条规则:
  { Tool: "name", Pattern: "substring", Strength: 0.0-1.0 }

三类触发器:
  1. 命令 (Strength 0.90-0.98): "docker ps", "df -h", "systemctl"
  2. 工具名 (Strength 0.90-0.95): 用户直接说出工具名
  3. 关键词 (Strength 0.60-0.90): 工具特定词，低歧义

匹配策略:
  - 所有规则 OR 匹配（不互斥）
  - 同工具取最高 Strength
  - Strength ≥ 0.70 才视为命中
```

### 手写规则规模

- 初始: ~70 条（覆盖 20 个工具）
- 目标: ~200 条（覆盖所有工具 × 5-10 种常见说法）

## Stage 2 嵌入流程

```
1. 加载模型（一次）
2. 嵌入工具描述（一次，缓存 DB）
3. 每条用户消息:
   a. 推理 → 512维向量
   b. 与 20 个工具向量算余弦
   c. 排序 → Top-K
   d. 品类聚合（file/system/search/mcp/docker/dns）
   e. 输出: 工具级 Top-3 + 品类级 Top-1
```

## 词汇表

| 缩写 | 全称 |
|---|---|
| TF | Term Frequency（词频匹配） |
| BGE | BAAI General Embedding |
| ONNX | Open Neural Network Exchange |
| MTEB | Massive Text Embedding Benchmark |
