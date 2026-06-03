// Package database 统一数据访问层入口。
//
// 消费者只需导入此包即可使用所有数据仓库接口和类型，无需关心底层实现。
//
//	import "github.com/Yunqingqingxi/yunxi-home/internal/database"
//	var repo database.DomainRepository = database.NewDomainRepo(db)
package database

import (
	"github.com/Yunqingqingxi/yunxi-home/internal/database/base"
)

// ── 基础接口 ─────────────────────────────────────────────────

// Executor 最小数据库操作集合
type Executor = base.Executor

// ── 仓库接口 ─────────────────────────────────────────────────

// DomainRepository 域名记录仓库接口
type DomainRepository = base.DomainRepository

// HistoryRepository 历史记录仓库接口
type HistoryRepository = base.HistoryRepository

// UserRepository 用户仓库接口
type UserRepository = base.UserRepository

// ChatSessionRepository 聊天会话仓库接口
type ChatSessionRepository = base.ChatSessionRepository

// ConfigRepository 配置存储接口
type ConfigRepository = base.ConfigRepository

// FilePermissionRepository 文件权限仓库接口
type FilePermissionRepository = base.FilePermissionRepository

// ShareRepository 分享数据仓库接口
type ShareRepository = base.ShareRepository

// GoalRepository 目标仓库接口
type GoalRepository = base.GoalRepository

// TodoRepository 待办事项仓库接口
type TodoRepository = base.TodoRepository

// PromptRepository 提示词仓库接口
type PromptRepository = base.PromptRepository

// PromptRecord 提示词记录
type PromptRecord = base.PromptRecord

// ── 共享类型 ─────────────────────────────────────────────────

// ListParams 分页查询参数
type ListParams = base.ListParams

// ListResult 分页查询结果
type ListResult = base.ListResult

// HistoryStats 每日聚合统计
type HistoryStats = base.HistoryStats
