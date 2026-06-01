# NAS / 权限 / 终端 — 现状分析

> 2026-05-30

## 1. NAS 模块内部状态

### FileService 结构
```
FileService {
    rootDir     string         # 文件系统根 (沙箱 = sandboxRoot, 人类 = config 的 root_dir)
    allowedDirs []string       # 白名单 (沙箱只有一条: sandboxRoot, 人类来自 config)
    sandboxRoot string         # 非空 = 沙箱模式, "" = 非沙箱模式
    chunkMu     sync.Mutex    # 分片上传锁
}
```

**关键事实：**
- `resolve()` 是唯一权限入口——所有路径操作都经过它
- 沙箱模式：三重校验（拒绝 `..`、拼接 sandboxRoot、边界检查 `HasPrefix`）
- 非沙箱模式：遍历 `allowedDirs` 做 `HasPrefix` 匹配，无匹配则拒绝
- `allowedDirs` 目前实测等于 `[rootDir]`（New() 时如果传空则兜底为 `[rootDir]`）
- `rootDir` 在沙箱模式下被设为 `sandboxRoot`，在人类模式下来自 config

**FileService 对外方法（28 个）：**
- 基本 CRUD：ListDir, Mkdir, Delete, Rename, OpenFile, SaveFile, Exists
- 增强：SearchFiles, CopyFile, DirSize
- 分片：InitChunkUpload, SaveChunk, CompleteChunkUpload, GetChunkStatus, AbortChunkUpload, CleanExpiredChunks
- 磁盘/沙箱：GetDiskInfo, SandboxInfo
- 工具：RelPath（隐藏绝对路径）, metaPath, cleanChunkDirIfEmpty

### NAS 配置
```yaml
nas:
  enabled: bool          # 全局开关: 管整个文件模块是否启动
  root_dir: "/"          # 人类用户可见的文件系统根
  sandbox_root: "..."    # AI 沙箱隔离区，非空时人类用户也用沙箱（设计如此？）
  allowed_dirs: []       # 白名单，但目前未暴露给用户配置，仅作兜底
```

### wireserver 中的 NAS 初始化
```go
if sandboxFS != nil {
    fileFS = sandboxFS       // ← 有沙箱时，人类用户也用沙箱
} else {
    fileFS = nas.New(cfg.NAS.RootDir, cfg.NAS.AllowedDirs)  // ← 无沙箱用 root_dir
}
```

**问题：** `sandboxFS != nil` 时所有用户（包括管理员）都共享同一个沙箱 FileService。管理员的"全系统访问"能力被沙箱限制了。

---

## 2. 与用户权限的耦合度

### 权限检查链路
```
HTTP Request
  → JWTAuth (Echo 中间件, 验证 token)
  → FileAccess (文件权限中间件)
       ├── 提取 claims
       ├── admin? → 全部放行
       ├── 提取 filePath
       ├── /home/{username}/ 路径? → 自动放行
       └── 查 file_permissions 表 (30s 缓存)
            ├── 匹配 → 检查 can_read/can_write/can_delete
            └── 无匹配 → 403
  → Handler (如 FilesHandler.ListFiles)
  → FileService.ListDir(path)
       ├── resolve(path) → 沙箱/白名单检查
       └── os.ReadDir + dirSize
```

### 耦合点
1. **FileAccess 中间件依赖 `permRepo` + `userRepo`** — 每个文件请求查一次 DB（缓存 30s）
2. **FileAccess 与 FileService 完全解耦** — 中间件只管"能不能访问这个路径"，FileService 不管"谁在访问"
3. **沙箱与权限表是两层独立的防护** — 沙箱限制物理边界，权限表限制逻辑边界
4. **管理员完全绕行所有检查** — admin role → 直接 next()
5. **`allowed_dirs` 未与 `file_permissions` 整合** — 两套独立的"你能去哪"规则

### 耦合度评分
- 与用户权限系统：**中等**（通过 FileAccess 中间件单向依赖，FileService 本身无感知）
- 与 Auth 系统：**低**（仅通过 JWT claims 提取 username/role）
- 与数据库：**低**（权限表查询，但 FileService 用的是文件系统）

---

## 3. 终端模块现状

```go
// terminal/terminal.go — 82 行
type TerminalHandler struct {
    enabled bool
}
func NewHandler(enabled bool) *TerminalHandler
func (h *TerminalHandler) Handle(c echo.Context) error {
    // WebSocket upgrade → pty.Start
}
```

**终端与权限系统的耦合度：零。**
- 不检查角色——任何登录用户都能拿到 shell
- 不查权限表——拿到 shell 就是用户级别的系统访问
- 仅靠 JWT 认证——通过 Echo 的 /api group 中间件保护，但除此无其他限制

---

## 4. 总结

```
           ┌─────────────┐
           │   Auth/JWT  │
           └──────┬──────┘
                  │ 验证身份
     ┌────────────┼────────────┐
     │            │            │
     ▼            ▼            ▼
┌─────────┐ ┌──────────┐ ┌──────────┐
│  终端    │ │ 文件权限  │ │   NAS    │
│ enabled │ │ FileAccess│ │ FileSvc  │
│ 无权限   │ │ permRepo │ │ resolve  │
│ 无沙箱   │ │ userRepo │ │ allowed  │
│         │ │          │ │ sandbox  │
│ 零耦合   │ │ 中等耦合  │ │ 独立模块  │
└─────────┘ └──────────┘ └──────────┘
     0          中           低
```

| 模块 | 对用户身份的感知 | 对权限的控制 | 对沙箱的感知 | 建议 |
|------|----------------|-------------|-------------|------|
| 终端 | 仅 JWT | 无 | 无 | 至少加 admin-only |
| 文件权限 | username + role | can_r/w/d/s | 无 | 与 NAS allowed_dirs 统一 |
| NAS | 无（FileService 不知道谁在调用） | 无（靠中间件） | 沙箱=物理隔离 | 把 allowed_dirs 移到权限表 |
| 分享 | 无（公开链接） | 仅密码保护 | 继承 FileService | 可以加"分享给特定用户" |
