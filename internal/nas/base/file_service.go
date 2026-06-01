// Package base 定义 NAS 文件系统模块的通用类型和接口，零外部依赖。
package base

import (
	"io"
	"io/fs"
	"time"
)

// FileInfo 文件/目录信息
type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	ModTime time.Time `json:"mod_time"`
	Ext     string    `json:"ext,omitempty"`
}

// FileStat 文件属性（含权限和所有者）
type FileStat struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	IsDir       bool   `json:"is_dir"`
	ModTime     string `json:"mod_time"`
	Mode        string `json:"mode"`
	Permissions string `json:"permissions"`
	Owner       string `json:"owner"`
	Group       string `json:"group"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Path    string  `json:"path"`
	Total   uint64  `json:"total"`
	Free    uint64  `json:"free"`
	Used    uint64  `json:"used"`
	UsedPct float64 `json:"used_pct"`
}

// ChunkMeta 分片上传元数据
type ChunkMeta struct {
	UploadID    string `json:"upload_id"`
	Filename    string `json:"filename"`
	Dir         string `json:"dir"`
	TotalSize   int64  `json:"total_size"`
	ChunkSize   int64  `json:"chunk_size"`
	TotalChunks int    `json:"total_chunks"`
	ChunksDone  []bool `json:"chunks_done"`
	CreatedAt   int64  `json:"created_at"`
}

// Share 分享记录
type Share struct {
	ID        int64     `json:"id"`
	Token     string    `json:"token"`
	FilePath  string    `json:"file_path"`
	Password  string    `json:"-"` // hashed, never returned
	HasPass   bool      `json:"has_pass"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	Downloads int64     `json:"downloads"`
}

// FileService 文件服务接口，所有文件操作均通过此接口。
type FileService interface {
	// 基本信息
	IsSandbox() bool
	SandboxRoot() string
	SandboxInfo() map[string]any

	// 目录操作
	ListDir(requestPath string) ([]FileInfo, error)
	Mkdir(requestPath string) error

	// 文件属性
	StatFile(requestPath string) (*FileStat, error)

	// 文件操作
	Delete(requestPath string) error
	Rename(oldPath, newPath string) error
	CopyFile(srcPath, dstPath string) error
	OpenFile(requestPath string) (io.ReadCloser, fs.FileInfo, error)
	SaveFile(dirPath, filename string, reader io.Reader) (string, error)
	WriteFile(requestPath string, data []byte) error
	CreateZip(w io.Writer, paths []string) error
	Exists(requestPath string) bool

	// 搜索
	SearchFiles(requestPath, query string, recursive bool, maxDepth int) ([]FileInfo, error)

	// 路径工具
	RelPath(absPath string) string

	// 分片上传
	InitChunkUpload(dir, filename string, totalSize, chunkSize int64) (*ChunkMeta, error)
	SaveChunk(uploadID string, chunkIndex int, reader io.Reader) error
	CompleteChunkUpload(uploadID string) (string, error)
	GetChunkStatus(uploadID string) (*ChunkMeta, error)
	AbortChunkUpload(uploadID string) error
	CleanExpiredChunks()
	StartChunkGC(interval time.Duration)
	MetaPath(uploadID string) string

	// 磁盘信息
	GetDiskInfo(requestPath string) (*DiskInfo, error)
}
