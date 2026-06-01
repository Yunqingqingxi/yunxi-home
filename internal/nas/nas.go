// Package nas 统一 NAS 文件系统模块入口。
//
// 消费者只需导入此包即可使用所有文件操作功能，无需关心底层实现。
//
//	import "github.com/Yunqingqingxi/yunxi-home/internal/nas"
//	fs := nas.New("/mnt/data", []string{"/mnt/data"})
//	files, _ := fs.ListDir("/")
package nas

import (
	"github.com/Yunqingqingxi/yunxi-home/internal/nas/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/nas/local"
)

// ── 类型别名（消费者直接使用 nas.FileInfo, nas.FileService 等）─────────

// FileInfo 文件/目录信息
type FileInfo = base.FileInfo

// DiskInfo 磁盘信息
type DiskInfo = base.DiskInfo

// ChunkMeta 分片上传元数据
type ChunkMeta = base.ChunkMeta

// Share 分享记录
type Share = base.Share

// FileService 文件服务接口
type FileService = base.FileService

// ── 工厂函数 ─────────────────────────────────────────────────

// New 创建文件服务（无沙箱，用于人类用户）
func New(rootDir string, allowedDirs []string) FileService {
	return local.New(rootDir, allowedDirs)
}

// NewSandbox 创建沙箱文件服务 — 所有路径操作强制限制在 sandboxRoot 内
func NewSandbox(sandboxRoot string) FileService {
	return local.NewSandbox(sandboxRoot)
}
