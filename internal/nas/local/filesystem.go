// Package local 提供本地文件系统实现，实现 base.FileService 接口。
package local

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"log/slog"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Yunqingqingxi/yunxi-home/internal/nas/base"
)

// FileSystem 本地文件系统实现
type FileSystem struct {
	rootDir     string
	allowedDirs []string
	sandboxRoot string   // 沙箱根目录，非空时强制所有操作在此目录下
	chunkMu     sync.Mutex // 保护分片元数据的并发读写
}

// 编译期检查接口实现
var _ base.FileService = (*FileSystem)(nil)

// New 创建文件服务（无沙箱，用于人类用户）
func New(rootDir string, allowedDirs []string) *FileSystem {
	if rootDir == "" {
		rootDir = "/"
	}
	if len(allowedDirs) == 0 {
		allowedDirs = []string{rootDir}
	}
	return &FileSystem{rootDir: rootDir, allowedDirs: allowedDirs}
}

// NewSandbox 创建沙箱文件服务 — 所有路径操作强制限制在 sandboxRoot 内
func NewSandbox(sandboxRoot string) *FileSystem {
	if sandboxRoot == "" {
		if runtime.GOOS == "windows" {
			home, _ := os.UserHomeDir()
			sandboxRoot = filepath.Join(home, ".yunxi", "data", "yunxiFiles")
		} else {
			sandboxRoot = "/opt/yunxi-home/data/yunxiFiles"
		}
	}
	absRoot, _ := filepath.Abs(sandboxRoot)
	return &FileSystem{
		rootDir:     absRoot,
		allowedDirs: []string{absRoot},
		sandboxRoot: absRoot,
	}
}

// IsSandbox 是否运行在沙箱模式
func (s *FileSystem) IsSandbox() bool { return s.sandboxRoot != "" }

// SandboxRoot 返回沙箱根目录（仅调试用）
func (s *FileSystem) SandboxRoot() string { return s.sandboxRoot }

// resolve 解析并验证路径。
// 自动去除沙箱根目录前缀，防止路径双重嵌套。
func (s *FileSystem) resolve(requestPath string) (string, error) {
	// 去沙箱根前缀：/opt/.../yunxiFiles/pictures → /pictures
	if s.sandboxRoot != "" {
		absSandbox := filepath.Clean(s.sandboxRoot)
		absRequest := filepath.Clean(requestPath)
		if strings.HasPrefix(absRequest, absSandbox) {
			requestPath = strings.TrimPrefix(absRequest, absSandbox)
			if requestPath == "" {
				requestPath = "/"
			}
		}
	}
	clean := filepath.Clean(requestPath)

	// ── 沙箱模式 ──
	if s.sandboxRoot != "" {
		if strings.Contains(requestPath, "..") {
			return "", fmt.Errorf("沙箱拒绝: 非法路径组件")
		}
		if strings.Contains(requestPath, ":") {
			return "", fmt.Errorf("沙箱拒绝: 非法路径 (包含 Windows 盘符)")
		}

		if clean == string(filepath.Separator) || clean == "." {
			return filepath.Clean(s.sandboxRoot), nil
		}

		rel := clean
		if len(rel) > 0 && rel[0] == filepath.Separator {
			rel = rel[1:]
		}
		joined := filepath.Join(s.sandboxRoot, rel)
		abs, err := filepath.Abs(joined)
		if err != nil {
			return "", fmt.Errorf("沙箱路径解析失败: %w", err)
		}
		abs = filepath.Clean(abs)
		absSandbox := filepath.Clean(s.sandboxRoot)

		if abs != absSandbox && !strings.HasPrefix(abs, absSandbox+string(filepath.Separator)) {
			return "", fmt.Errorf("沙箱拒绝: 路径逃逸 %s", filepath.ToSlash(requestPath))
		}
		return abs, nil
	}

	// ── 非沙箱模式 ──
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("路径解析失败: %w", err)
	}
	allowed := false
	for _, dir := range s.allowedDirs {
		absDir, _ := filepath.Abs(dir)
		absDir = filepath.Clean(absDir)
		if strings.HasPrefix(abs, absDir) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("访问被拒绝: 路径不在允许范围内")
	}
	return abs, nil
}

// ListDir 列出目录内容
func (s *FileSystem) ListDir(requestPath string) ([]base.FileInfo, error) {
	slog.Info("列出目录", "路径", requestPath)
	absPath, err := s.resolve(requestPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var result []base.FileInfo
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fi := base.FileInfo{
			Name:    entry.Name(),
			Path:    filepath.ToSlash(filepath.Join(requestPath, entry.Name())),
			Size:    info.Size(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime(),
		}
		if entry.IsDir() {
			fi.Size = s.dirSize(filepath.Join(absPath, entry.Name()))
		}
		if !entry.IsDir() {
			fi.Ext = strings.ToLower(filepath.Ext(entry.Name()))
		}
		result = append(result, fi)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

func (s *FileSystem) dirSize(absPath string) int64 {
	var total int64
	count := 0
	maxEntries := 5000
	filepath.WalkDir(absPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == absPath {
			return nil
		}
		if count >= maxEntries {
			return filepath.SkipAll
		}
		if !d.IsDir() {
			if info, e := d.Info(); e == nil {
				total += info.Size()
			}
			count++
		}
		return nil
	})
	return total
}

// Mkdir 创建目录
func (s *FileSystem) Mkdir(requestPath string) error {
	absPath, err := s.resolve(requestPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	return nil
}

// Delete 删除文件或目录
func (s *FileSystem) Delete(requestPath string) error {
	slog.Info("删除文件", "路径", requestPath)
	absPath, err := s.resolve(requestPath)
	if err != nil {
		return err
	}
	if absPath == s.rootDir {
		return fmt.Errorf("不允许删除根目录")
	}
	if err := os.RemoveAll(absPath); err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	return nil
}

// Rename 重命名/移动文件
func (s *FileSystem) Rename(oldPath, newPath string) error {
	absOld, err := s.resolve(oldPath)
	if err != nil {
		return err
	}
	absNew, err := s.resolve(newPath)
	if err != nil {
		return err
	}
	if err := os.Rename(absOld, absNew); err != nil {
		return fmt.Errorf("重命名失败: %w", err)
	}
	return nil
}

// OpenFile 打开文件用于读取
func (s *FileSystem) OpenFile(requestPath string) (io.ReadCloser, fs.FileInfo, error) {
	absPath, err := s.resolve(requestPath)
	if err != nil {
		return nil, nil, err
	}
	stat, err := os.Stat(absPath)
	if err != nil {
		return nil, nil, fmt.Errorf("文件不存在: %w", err)
	}
	if stat.IsDir() {
		return nil, nil, fmt.Errorf("不能下载目录")
	}
	f, err := os.Open(absPath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开文件失败: %w", err)
	}
	return f, stat, nil
}

// SaveFile 保存上传的文件
func (s *FileSystem) SaveFile(dirPath, filename string, reader io.Reader) (string, error) {
	absDir, err := s.resolve(dirPath)
	if err != nil {
		return "", err
	}
	filename = strings.ReplaceAll(filename, "\\", "/")
	if idx := strings.LastIndex(filename, "/"); idx >= 0 {
		filename = filename[idx+1:]
	}
	if idx := strings.Index(filename, ":"); idx >= 0 {
		filename = filename[idx+1:]
	}
	filename = strings.TrimSpace(filename)
	if filename == "." || filename == "" {
		return "", fmt.Errorf("无效的文件名")
	}
	fullPath := filepath.Join(absDir, filename)
	// 自动创建父目录，支持文件夹上传
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("创建父目录失败: %w", err)
	}
	f, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, reader); err != nil {
		os.Remove(fullPath)
		return "", fmt.Errorf("写入文件失败: %w", err)
	}
	return fullPath, nil
}

// RelPath 将绝对路径转为相对路径
func (s *FileSystem) RelPath(absPath string) string {
	p := strings.TrimPrefix(absPath, s.rootDir)
	p = strings.TrimPrefix(p, string(filepath.Separator))
	if p == "" {
		return "/"
	}
	return filepath.ToSlash("/" + p)
}

// Exists 检查路径是否存在
func (s *FileSystem) Exists(requestPath string) bool {
	absPath, err := s.resolve(requestPath)
	if err != nil {
		return false
	}
	_, err = os.Stat(absPath)
	return err == nil
}

// SearchFiles 搜索文件（支持递归搜索）
func (s *FileSystem) SearchFiles(requestPath, query string, recursive bool, maxDepth int) ([]base.FileInfo, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if maxDepth > 10 {
		maxDepth = 10
	}
	query = strings.ToLower(query)
	maxResults := 500

	absPath, err := s.resolve(requestPath)
	if err != nil {
		return nil, err
	}

	var results []base.FileInfo
	if !recursive {
		files, err := s.ListDir(requestPath)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			if strings.Contains(strings.ToLower(f.Name), query) {
				results = append(results, f)
			}
		}
	} else {
		err := filepath.WalkDir(absPath, func(walkPath string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if walkPath == absPath {
				return nil
			}
			rel, _ := filepath.Rel(absPath, walkPath)
			depth := len(strings.Split(rel, string(filepath.Separator)))
			if depth > maxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.Contains(strings.ToLower(d.Name()), query) {
				return nil
			}
			if len(results) >= maxResults {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			fi := base.FileInfo{
				Name:    d.Name(),
				Path:    filepath.ToSlash(filepath.Join(requestPath, rel)),
				Size:    info.Size(),
				IsDir:   d.IsDir(),
				ModTime: info.ModTime(),
			}
			if !d.IsDir() {
				fi.Ext = strings.ToLower(filepath.Ext(d.Name()))
			}
			results = append(results, fi)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("搜索失败: %w", err)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].IsDir != results[j].IsDir {
			return results[i].IsDir
		}
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return results, nil
}

// CopyFile 复制文件或目录
func (s *FileSystem) CopyFile(srcPath, dstPath string) error {
	absSrc, err := s.resolve(srcPath)
	if err != nil {
		return err
	}
	absDst, err := s.resolve(dstPath)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(absSrc)
	if err != nil {
		return fmt.Errorf("源文件不存在: %w", err)
	}

	if srcInfo.IsDir() {
		return s.copyDir(absSrc, absDst)
	}
	return s.copySingleFile(absSrc, absDst, srcInfo)
}

func (s *FileSystem) copySingleFile(src, dst string, srcInfo os.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		os.Remove(dst)
		return fmt.Errorf("复制文件失败: %w", err)
	}
	return nil
}

func (s *FileSystem) copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %w", err)
	}

	for _, entry := range entries {
		srcChild := filepath.Join(src, entry.Name())
		dstChild := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := s.copyDir(srcChild, dstChild); err != nil {
				return err
			}
		} else {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := s.copySingleFile(srcChild, dstChild, info); err != nil {
				return err
			}
		}
	}
	return nil
}

// SandboxInfo 返回沙箱状态信息
func (s *FileSystem) SandboxInfo() map[string]any {
	return map[string]any{
		"sandbox":      s.sandboxRoot != "",
		"sandbox_root": s.sandboxRoot,
		"root_dir":     s.rootDir,
	}
}

// ── 分片上传 ──────────────────────────────────────────

func (s *FileSystem) chunkDir() string {
	return filepath.Join(s.rootDir, ".chunks")
}

func (s *FileSystem) metaPath(uploadID string) string {
	return filepath.Join(s.chunkDir(), uploadID+".meta")
}

// MetaPath returns the meta file path for an upload (exported for tests)
func (s *FileSystem) MetaPath(uploadID string) string {
	return s.metaPath(uploadID)
}

// InitChunkUpload 初始化分片上传
func (s *FileSystem) InitChunkUpload(dir, filename string, totalSize, chunkSize int64) (*base.ChunkMeta, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("生成上传ID失败: %w", err)
	}
	uploadID := hex.EncodeToString(b)

	filename = strings.ReplaceAll(filename, "\\", "/")
	if idx := strings.LastIndex(filename, "/"); idx >= 0 {
		filename = filename[idx+1:]
	}
	if idx := strings.Index(filename, ":"); idx >= 0 {
		filename = filename[idx+1:]
	}
	filename = strings.TrimSpace(filename)
	if filename == "." || filename == "" {
		return nil, fmt.Errorf("无效的文件名")
	}

	absDir, err := s.resolve(dir)
	if err != nil {
		return nil, err
	}
	fullPath := filepath.Join(absDir, filename)

	f, err := os.Create(fullPath)
	if err != nil {
		return nil, fmt.Errorf("创建目标文件失败: %w", err)
	}
	if err := f.Truncate(totalSize); err != nil {
		f.Close()
		os.Remove(fullPath)
		return nil, fmt.Errorf("预分配磁盘空间失败: %w", err)
	}
	f.Close()

	totalChunks := int((totalSize + chunkSize - 1) / chunkSize)
	meta := &base.ChunkMeta{
		UploadID:    uploadID,
		Filename:    filename,
		Dir:         dir,
		TotalSize:   totalSize,
		ChunkSize:   chunkSize,
		TotalChunks: totalChunks,
		ChunksDone:  make([]bool, totalChunks),
		CreatedAt:   time.Now().Unix(),
	}

	os.MkdirAll(s.chunkDir(), 0755)
	metaFile := s.metaPath(uploadID)
	data, err := json.Marshal(meta)
	if err != nil {
		os.Remove(fullPath)
		return nil, err
	}
	if err := os.WriteFile(metaFile, data, 0644); err != nil {
		os.Remove(fullPath)
		return nil, fmt.Errorf("保存元数据失败: %w", err)
	}

	return meta, nil
}

// SaveChunk 将分片写入预分配目标文件的正确偏移位置
func (s *FileSystem) SaveChunk(uploadID string, chunkIndex int, reader io.Reader) error {
	metaFile := s.metaPath(uploadID)
	if _, err := os.Stat(metaFile); os.IsNotExist(err) {
		return fmt.Errorf("上传会话不存在")
	}

	s.chunkMu.Lock()
	meta, err := s.loadChunkMeta(uploadID)
	if err != nil {
		s.chunkMu.Unlock()
		return err
	}
	if chunkIndex < 0 || chunkIndex >= meta.TotalChunks {
		s.chunkMu.Unlock()
		return fmt.Errorf("分片索引无效: %d (总数: %d)", chunkIndex, meta.TotalChunks)
	}
	if meta.ChunksDone[chunkIndex] {
		s.chunkMu.Unlock()
		return fmt.Errorf("分片 %d 已上传过", chunkIndex)
	}
	absDir, err := s.resolve(meta.Dir)
	if err != nil {
		s.chunkMu.Unlock()
		return err
	}
	fullPath := filepath.Join(absDir, meta.Filename)
	s.chunkMu.Unlock()

	f, err := os.OpenFile(fullPath, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开目标文件失败: %w", err)
	}
	defer f.Close()

	offset := int64(chunkIndex) * meta.ChunkSize
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("定位写入位置失败: %w", err)
	}

	buf := make([]byte, 64*1024)
	if _, err := io.CopyBuffer(f, reader, buf); err != nil {
		return fmt.Errorf("写入分片失败: %w", err)
	}

	s.chunkMu.Lock()
	meta.ChunksDone[chunkIndex] = true
	err = s.saveChunkMeta(uploadID, meta)
	s.chunkMu.Unlock()
	return err
}

// CompleteChunkUpload 验证所有分片完成，清理 meta
func (s *FileSystem) CompleteChunkUpload(uploadID string) (string, error) {
	meta, err := s.loadChunkMeta(uploadID)
	if err != nil {
		return "", err
	}

	for i, done := range meta.ChunksDone {
		if !done {
			return "", fmt.Errorf("分片 %d 尚未上传", i)
		}
	}

	absDir, err := s.resolve(meta.Dir)
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(absDir, meta.Filename)

	stat, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("目标文件不存在")
	}
	if stat.Size() != meta.TotalSize {
		return "", fmt.Errorf("文件大小不匹配: 期望 %d, 实际 %d", meta.TotalSize, stat.Size())
	}

	os.Remove(s.metaPath(uploadID))
	s.cleanChunkDirIfEmpty()

	return fullPath, nil
}

// GetChunkStatus 获取上传进度
func (s *FileSystem) GetChunkStatus(uploadID string) (*base.ChunkMeta, error) {
	return s.loadChunkMeta(uploadID)
}

// AbortChunkUpload 中止并清理分片上传
func (s *FileSystem) AbortChunkUpload(uploadID string) error {
	meta, err := s.loadChunkMeta(uploadID)
	if err == nil {
		absDir, _ := s.resolve(meta.Dir)
		if absDir != "" {
			os.Remove(filepath.Join(absDir, meta.Filename))
		}
	}
	os.Remove(s.metaPath(uploadID))
	s.cleanChunkDirIfEmpty()
	return nil
}

// CleanExpiredChunks 清理过期的分片（超过 24 小时未完成）
func (s *FileSystem) CleanExpiredChunks() {
	chunkDir := s.chunkDir()
	entries, err := os.ReadDir(chunkDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Unix() - 86400
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta") {
			continue
		}
		uploadID := strings.TrimSuffix(entry.Name(), ".meta")
		meta, err := s.loadChunkMeta(uploadID)
		if err != nil || meta.CreatedAt < cutoff {
			s.AbortChunkUpload(uploadID)
		}
	}
	s.cleanChunkDirIfEmpty()
}

// StartChunkGC starts periodic cleanup of expired chunk uploads (>24h).
func (s *FileSystem) StartChunkGC(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.CleanExpiredChunks()
		}
	}()
	slog.Info("chunk GC started", "interval", interval)
}

func (s *FileSystem) cleanChunkDirIfEmpty() {
	chunkDir := s.chunkDir()
	entries, err := os.ReadDir(chunkDir)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		os.Remove(chunkDir)
	}
}

func (s *FileSystem) loadChunkMeta(uploadID string) (*base.ChunkMeta, error) {
	data, err := os.ReadFile(s.metaPath(uploadID))
	if err != nil {
		return nil, fmt.Errorf("上传会话不存在")
	}
	var meta base.ChunkMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("解析元数据失败: %w", err)
	}
	return &meta, nil
}

func (s *FileSystem) saveChunkMeta(uploadID string, meta *base.ChunkMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(s.metaPath(uploadID), data, 0644)
}

func (s *FileSystem) StatFile(requestPath string) (*base.FileStat, error) {
	absPath, err := s.resolve(requestPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	mode := info.Mode()
	perm := mode.Perm()
	st := &base.FileStat{
		Name:        info.Name(),
		Path:        requestPath,
		Size:        info.Size(),
		IsDir:       info.IsDir(),
		ModTime:     info.ModTime().Format("2006-01-02 15:04:05"),
		Mode:        mode.String(),
		Permissions: fmt.Sprintf("%04o", perm),
	}
	fillOwnerGroupFromSys(info, st)
	return st, nil
}

// WriteFile overwrites or creates a file with the given data.
func (s *FileSystem) WriteFile(requestPath string, data []byte) error {
	absPath, err := s.resolve(requestPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return fmt.Errorf("创建父目录失败: %w", err)
	}
	return os.WriteFile(absPath, data, 0644)
}

// CreateZip creates a zip archive of the given files and writes to w.
func (s *FileSystem) CreateZip(w io.Writer, paths []string) error {
	zw := zip.NewWriter(w)
	defer zw.Close()
	for _, p := range paths {
		if err := s.addFileToZip(zw, p); err != nil {
			slog.Warn("zip: skip file", "path", p, "error", err)
			continue
		}
	}
	return nil
}

func (s *FileSystem) addFileToZip(zw *zip.Writer, requestPath string) error {
	reader, info, err := s.OpenFile(requestPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	if info.IsDir() {
		return nil
	}
	name := filepath.Base(requestPath)
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	header.Method = zip.Deflate
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, reader)
	return err
}
