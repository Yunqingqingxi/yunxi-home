package handlers

import (
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/Yunqingqingxi/yunxi-home/internal/database"
	"github.com/Yunqingqingxi/yunxi-home/internal/nas"
	echomw "github.com/Yunqingqingxi/yunxi-home/internal/web/middleware"
)

// FilesHandler 文件管理 Handler
type FilesHandler struct {
	fs       nas.FileService
	userRepo database.UserRepository
}

// NewFilesHandler 创建文件 Handler
func NewFilesHandler(fs nas.FileService) *FilesHandler {
	return &FilesHandler{fs: fs, userRepo: nil} // userRepo set via WithUserRepo
}

// WithUserRepo injects user repository for quota checks
func (h *FilesHandler) WithUserRepo(repo database.UserRepository) *FilesHandler {
	h.userRepo = repo
	return h
}

// ListFiles 列出目录内容
// GET /api/nas/files?path=/
func (h *FilesHandler) ListFiles(c echo.Context) error {
	path := c.QueryParam("path")
	if path == "" {
		path = "/"
	}
	files, err := h.fs.ListDir(path)
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(files))
}

// DownloadFile 下载文件
// GET /api/nas/files/download?path=...
func (h *FilesHandler) DownloadFile(c echo.Context) error {
	path := c.QueryParam("path")
	if path == "" {
		return c.JSON(http.StatusBadRequest, errorResp("path 参数不能为空"))
	}

	reader, _, err := h.fs.OpenFile(path)
	if err != nil {
		return c.JSON(http.StatusNotFound, errorResp(err.Error()))
	}
	defer reader.Close()

	filename := filepath.Base(path)
	c.Response().Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.Response().Header().Set("Content-Type", "application/octet-stream")
	c.Response().WriteHeader(http.StatusOK)

	_, copyErr := io.Copy(c.Response(), reader)
	if copyErr != nil {
		return copyErr
	}
	return nil
}

// UploadFile 上传文件
// POST /api/nas/files/upload
func (h *FilesHandler) UploadFile(c echo.Context) error {
	ct := c.Request().Header.Get("Content-Type")

	log.Debug("upload request", "content-type", ct, "content-length", c.Request().Header.Get("Content-Length"))

	dir := c.FormValue("dir")
	// 清理 Windows 路径翻译产生的非法路径 (如 C:/Program Files/Git/ -> /)
	if !strings.HasPrefix(dir, "/") || strings.Contains(dir, ":") {
		dir = "/"
	}
	if dir == "" {
		dir = "/"
	}

	file, err := c.FormFile("file")
	if err != nil {
		log.Warn("upload parse failed", "error", err.Error(), "content-type", ct)
		return c.JSON(http.StatusBadRequest, map[string]any{"code": 400, "message": "请选择文件", "detail": err.Error(), "content_type": ct})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("读取文件失败"))
	}
	defer src.Close()

	// Quota check for non-admin users
	if h.userRepo != nil {
		if claims := getJWTAuthClaims(c); claims != nil && claims.Role != "admin" {
			dbUser, err := h.userRepo.GetByUsername(c.Request().Context(), claims.Username)
			if err == nil && dbUser.StorageQuota > 0 {
				if dbUser.StorageUsed+file.Size > dbUser.StorageQuota {
					return c.JSON(413, errorResp("存储配额不足"))
				}
			}
		}
	}

	savedPath, err := h.fs.SaveFile(dir, file.Filename, src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}

	return c.JSON(http.StatusOK, successResp(map[string]string{
		"message": "上传成功",
		"path":    h.fs.RelPath(savedPath),
	}))
}

// CreateDir 创建目录
// POST /api/nas/files/mkdir
func (h *FilesHandler) CreateDir(c echo.Context) error {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if req.Path == "" {
		return c.JSON(http.StatusBadRequest, errorResp("path 不能为空"))
	}
	if err := h.fs.Mkdir(req.Path); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "目录已创建"}))
}

// DeleteItem 删除文件或目录
// DELETE /api/nas/files
func (h *FilesHandler) DeleteItem(c echo.Context) error {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if req.Path == "" {
		return c.JSON(http.StatusBadRequest, errorResp("path 不能为空"))
	}
	if err := h.fs.Delete(req.Path); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "已删除"}))
}

// RenameItem 重命名/移动
// PUT /api/nas/files/rename
func (h *FilesHandler) RenameItem(c echo.Context) error {
	var req struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if err := h.fs.Rename(req.OldPath, req.NewPath); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "已重命名"}))
}

// GetDiskInfo 获取磁盘信息
// GET /api/nas/diskinfo?path=/
func (h *FilesHandler) GetDiskInfo(c echo.Context) error {
	path := c.QueryParam("path")
	if path == "" {
		path = "/"
	}
	info, err := h.fs.GetDiskInfo(path)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	info.Path = path // 隐藏服务端绝对路径
	return c.JSON(http.StatusOK, successResp(info))
}

// SearchFiles 搜索文件 (支持递归搜索)
// GET /api/nas/search?q=xxx&path=/&recursive=true&depth=3
func (h *FilesHandler) SearchFiles(c echo.Context) error {
	query := strings.ToLower(c.QueryParam("q"))
	path := c.QueryParam("path")
	if path == "" {
		path = "/"
	}
	if query == "" {
		return c.JSON(http.StatusBadRequest, errorResp("q 参数不能为空"))
	}

	recursive := c.QueryParam("recursive") == "true"
	depth := 3
	if v, err := strconv.Atoi(c.QueryParam("depth")); err == nil && v > 0 && v <= 10 {
		depth = v
	}

	results, err := h.fs.SearchFiles(path, query, recursive, depth)
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResp(err.Error()))
	}

	return c.JSON(http.StatusOK, successResp(results))
}

// CopyItem 复制文件或目录
// POST /api/nas/files/copy
func (h *FilesHandler) CopyItem(c echo.Context) error {
	var req struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if req.Src == "" || req.Dst == "" {
		return c.JSON(http.StatusBadRequest, errorResp("src 和 dst 不能为空"))
	}
	if err := h.fs.CopyFile(req.Src, req.Dst); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "已复制"}))
}

// BatchDeleteItems 批量删除文件或目录
// GET /api/nas/files/stat?path=
func (h *FilesHandler) StatFile(c echo.Context) error {
	path := c.QueryParam("path")
	if path == "" {
		return c.JSON(http.StatusBadRequest, errorResp("path 不能为空"))
	}
	stat, err := h.fs.StatFile(path)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(stat))
}

// POST /api/nas/files/batch-delete
func (h *FilesHandler) BatchDeleteItems(c echo.Context) error {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if len(req.Paths) == 0 {
		return c.JSON(http.StatusBadRequest, errorResp("paths 不能为空"))
	}
	if len(req.Paths) > 200 {
		return c.JSON(http.StatusBadRequest, errorResp("单次最多删除 200 项"))
	}

	type result struct {
		Path    string `json:"path"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}
	var results []result
	for _, p := range req.Paths {
		r := result{Path: p}
		if err := h.fs.Delete(p); err != nil {
			r.Error = err.Error()
		} else {
			r.Success = true
		}
		results = append(results, r)
	}

	return c.JSON(http.StatusOK, successResp(map[string]interface{}{
		"results": results,
		"total":   len(results),
	}))
}
// StreamFile 视频/音频流播放，支持 HTTP Range 请求（Seeking）
// GET /api/nas/files/stream?path=/videos/demo.mp4
func (h *FilesHandler) StreamFile(c echo.Context) error {
	reqPath := c.QueryParam("path")
	if reqPath == "" {
		return c.JSON(http.StatusBadRequest, errorResp("path 参数不能为空"))
	}

	reader, stat, err := h.fs.OpenFile(reqPath)
	if err != nil {
		return c.JSON(http.StatusNotFound, errorResp(err.Error()))
	}
	defer reader.Close()

	fileSize := stat.Size()
	filename := filepath.Base(reqPath)

	ext := strings.ToLower(filepath.Ext(filename))
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 只对视频/音频启用内联播放
	if strings.HasPrefix(contentType, "video/") || strings.HasPrefix(contentType, "audio/") {
		c.Response().Header().Set("Content-Type", contentType)
	} else {
		c.Response().Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
		c.Response().Header().Set("Content-Type", "application/octet-stream")
		c.Response().WriteHeader(http.StatusOK)
		_, copyErr := io.Copy(c.Response(), reader)
		return copyErr
	}

	// HTTP Range 处理 (支持视频 seeking)
	rangeHeader := c.Request().Header.Get("Range")
	if rangeHeader == "" {
		c.Response().Header().Set("Accept-Ranges", "bytes")
		c.Response().Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))
		c.Response().WriteHeader(http.StatusOK)
		_, copyErr := io.Copy(c.Response(), reader)
		return copyErr
	}

	rangeStr := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.SplitN(rangeStr, "-", 2)
	if len(parts) != 2 {
		c.Response().WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return nil
	}

	start, err1 := strconv.ParseInt(parts[0], 10, 64)
	if err1 != nil {
		start = 0
	}

	var end int64
	if parts[1] != "" {
		end, err1 = strconv.ParseInt(parts[1], 10, 64)
		if err1 != nil || end >= fileSize {
			end = fileSize - 1
		}
	} else {
		// Range: bytes=0- (open-ended) — 只发首批 8MB, 浏览器会自动请求后续
		end = start + 8*1024*1024 - 1
		if end >= fileSize {
			end = fileSize - 1
		}
	}

	if start > end || start >= fileSize {
		c.Response().Header().Set("Content-Range", "bytes */"+strconv.FormatInt(fileSize, 10))
		c.Response().WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return nil
	}

	chunkSize := end - start + 1

	if seeker, ok := reader.(io.ReadSeeker); ok {
		if _, err := seeker.Seek(start, io.SeekStart); err != nil {
			return c.JSON(http.StatusInternalServerError, errorResp("seek 失败"))
		}
	} else {
		if _, err := io.CopyN(io.Discard, reader, start); err != nil {
			return c.JSON(http.StatusInternalServerError, errorResp("跳过字节失败"))
		}
	}

	c.Response().Header().Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(fileSize, 10))
	c.Response().Header().Set("Content-Length", strconv.FormatInt(chunkSize, 10))
	c.Response().Header().Set("Content-Type", contentType)
	c.Response().Header().Set("Accept-Ranges", "bytes")
	c.Response().WriteHeader(http.StatusPartialContent)

	_, copyErr := io.CopyN(c.Response(), reader, chunkSize)
	return copyErr
}

// PreviewFile 文件预览 — 根据扩展名返回文本或内联图片
// GET /api/nas/files/preview?path=/docs/readme.txt
func (h *FilesHandler) PreviewFile(c echo.Context) error {
	reqPath := c.QueryParam("path")
	if reqPath == "" {
		return c.JSON(http.StatusBadRequest, errorResp("path 参数不能为空"))
	}

	reader, stat, err := h.fs.OpenFile(reqPath)
	if err != nil {
		return c.JSON(http.StatusNotFound, errorResp(err.Error()))
	}
	defer reader.Close()

	ext := strings.ToLower(filepath.Ext(reqPath))
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "text/plain; charset=utf-8"
	}

	c.Response().Header().Set("Content-Type", contentType)
	c.Response().Header().Set("X-File-Size", strconv.FormatInt(stat.Size(), 10))

	// 限制预览大小: 最大 4MB
	maxSize := int64(4 * 1024 * 1024)
	if stat.Size() > maxSize {
		limitReader := io.LimitReader(reader, maxSize)
		_, copyErr := io.Copy(c.Response(), limitReader)
		return copyErr
	}

	_, copyErr := io.Copy(c.Response(), reader)
	return copyErr
}
// ── 分片上传 ──────────────────────────────────────────

// InitChunkUpload 初始化分片上传
// POST /api/nas/files/upload/init
func (h *FilesHandler) InitChunkUpload(c echo.Context) error {
	var req struct {
		Filename  string `json:"filename"`
		Dir       string `json:"dir"`
		TotalSize int64  `json:"total_size"`
		ChunkSize int64  `json:"chunk_size"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if req.Filename == "" {
		return c.JSON(http.StatusBadRequest, errorResp("filename 不能为空"))
	}
	if req.Dir == "" {
		req.Dir = "/"
	}
	if req.ChunkSize <= 0 {
		req.ChunkSize = 5 * 1024 * 1024 // 默认 5MB
	}
	if req.ChunkSize > 50*1024*1024 {
		req.ChunkSize = 50 * 1024 * 1024 // 最大 50MB/片
	}
	if req.TotalSize <= 0 {
		return c.JSON(http.StatusBadRequest, errorResp("total_size 必须大于 0"))
	}

	meta, err := h.fs.InitChunkUpload(req.Dir, req.Filename, req.TotalSize, req.ChunkSize)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(meta))
}

// SaveChunk 保存分片
// POST /api/nas/files/upload/chunk
func (h *FilesHandler) SaveChunk(c echo.Context) error {
	uploadID := c.FormValue("upload_id")
	chunkIdx := c.FormValue("chunk_index")
	if uploadID == "" || chunkIdx == "" {
		return c.JSON(http.StatusBadRequest, errorResp("upload_id 和 chunk_index 不能为空"))
	}
	chunkIndex, err := strconv.Atoi(chunkIdx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("chunk_index 无效"))
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("请选择文件分片"))
	}
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("读取分片失败"))
	}
	defer src.Close()

	if err := h.fs.SaveChunk(uploadID, chunkIndex, src); err != nil {
		log.Error("chunk save failed", "upload_id", uploadID, "chunk", chunkIndex, "error", err)
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]any{
		"upload_id":   uploadID,
		"chunk_index": chunkIndex,
		"status":      "ok",
	}))
}

// CompleteChunkUpload 合并分片，完成上传
// POST /api/nas/files/upload/complete
func (h *FilesHandler) CompleteChunkUpload(c echo.Context) error {
	var req struct {
		UploadID string `json:"upload_id"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if req.UploadID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("upload_id 不能为空"))
	}

	savedPath, err := h.fs.CompleteChunkUpload(req.UploadID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{
		"message": "上传完成",
		"path":    h.fs.RelPath(savedPath),
	}))
}

// GetChunkStatus 获取上传进度
// GET /api/nas/files/upload/status?upload_id=xxx
func (h *FilesHandler) GetChunkStatus(c echo.Context) error {
	uploadID := c.QueryParam("upload_id")
	if uploadID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("upload_id 不能为空"))
	}
	meta, err := h.fs.GetChunkStatus(uploadID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(meta))
}

// AbortChunkUpload 中止上传
// POST /api/nas/files/upload/abort
func (h *FilesHandler) AbortChunkUpload(c echo.Context) error {
	var req struct {
		UploadID string `json:"upload_id"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("参数错误"))
	}
	if req.UploadID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("upload_id 不能为空"))
	}
	if err := h.fs.AbortChunkUpload(req.UploadID); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "已清理"}))
}

// getJWTAuthClaims extracts JWT claims from Echo context.
// Uses interface to avoid jwt v3/v5 package conflict.
func getJWTAuthClaims(c echo.Context) *echomw.Claims {
	user := c.Get("user")
	if user == nil {
		return nil
	}
	type hasClaims interface{ Claims() interface{} }
	if t, ok := user.(hasClaims); ok {
		if c2, ok := t.Claims().(*echomw.Claims); ok {
			return c2
		}
	}
	return nil
}

// SaveFileContent saves edited text content back to a file.
// PUT /api/nas/files/save
func (h *FilesHandler) SaveFileContent(c echo.Context) error {
	path := c.QueryParam("path")
	if path == "" {
		return c.JSON(http.StatusBadRequest, errorResp("path 参数不能为空"))
	}
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, errorResp("读取请求体失败"))
	}
	if err := h.fs.WriteFile(path, body); err != nil {
		log.Error("保存文件失败", "path", path, "error", err)
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "已保存"}))
}

// GetFileTree returns a recursive directory tree.
// GET /api/nas/files/tree?path=/
func (h *FilesHandler) GetFileTree(c echo.Context) error {
	path := c.QueryParam("path")
	if path == "" { path = "/" }
	tree, err := h.buildTree(path, 5)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(tree))
}

func (h *FilesHandler) buildTree(dir string, maxDepth int) ([]map[string]any, error) {
	if maxDepth <= 0 { return nil, nil }
	entries, err := h.fs.ListDir(dir)
	if err != nil { return nil, err }
	var result []map[string]any
	for _, e := range entries {
		if !e.IsDir { continue }
		node := map[string]any{
			"name": e.Name, "path": e.Path,
		}
		children, _ := h.buildTree(e.Path, maxDepth-1)
		if children != nil { node["children"] = children }
		result = append(result, node)
	}
	return result, nil
}

// MoveFile moves/renames a file.
// POST /api/nas/files/move
func (h *FilesHandler) MoveFile(c echo.Context) error {
	var req struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	}
	if err := c.Bind(&req); err != nil || req.Src == "" || req.Dst == "" {
		return c.JSON(http.StatusBadRequest, errorResp("请提供 src 和 dst"))
	}
	if err := h.fs.Rename(req.Src, req.Dst); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp(err.Error()))
	}
	return c.JSON(http.StatusOK, successResp(map[string]string{"message": "已移动"}))
}

// BatchDownload creates a zip of multiple files and streams it.
// POST /api/nas/files/batch-download
func (h *FilesHandler) BatchDownload(c echo.Context) error {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.Bind(&req); err != nil || len(req.Paths) == 0 {
		return c.JSON(http.StatusBadRequest, errorResp("请提供文件路径列表"))
	}
	c.Response().Header().Set("Content-Type", "application/zip")
	c.Response().Header().Set("Content-Disposition", "attachment; filename=\"files.zip\"")
	writer := c.Response().Writer
	if err := h.fs.CreateZip(writer, req.Paths); err != nil {
		log.Error("zip打包失败", "error", err)
		return c.JSON(http.StatusInternalServerError, errorResp("打包失败"))
	}
	return nil
}
