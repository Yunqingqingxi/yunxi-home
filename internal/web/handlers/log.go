package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/yxd/yunxi-home/internal/ai"
)

const maxSystemLogLines = 10000 // 单次读取最大行数，防止内存爆炸

// LogHandler serves chat and system log APIs.
type LogHandler struct {
	chatLogger   *ai.ChatLogger
	systemLogDir string
}

func NewLogHandler(cl *ai.ChatLogger, sysLogDir string) *LogHandler {
	return &LogHandler{chatLogger: cl, systemLogDir: sysLogDir}
}

// ── Chat Logs ──────────────────────────────────────────────────────

// ListChatSessions returns metadata for all chat session log files.
// GET /api/logs/chat/sessions
func (h *LogHandler) ListChatSessions(c echo.Context) error {
	if h.chatLogger == nil || !h.chatLogger.Enabled() {
		return c.JSON(http.StatusOK, successResp(map[string]any{"sessions": []any{}}))
	}

	infos := h.chatLogger.ListSessions()
	// Estimate rounds by quickly scanning each file
	for i := range infos {
		if infos[i].Size < 10*1024 {
			infos[i].Rounds = countRounds(infos[i].File, h.chatLogger.Dir())
		}
	}

	return c.JSON(http.StatusOK, successResp(map[string]any{"sessions": infos}))
}

// GetChatLog returns chat session log events with filtering, sorting, and pagination.
// GET /api/logs/chat/:id?order=desc&type=error,tool_call&search=timeout&limit=200&offset=0
func (h *LogHandler) GetChatLog(c echo.Context) error {
	sessionID := c.Param("id")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session ID"))
	}

	events, err := h.chatLogger.GetSessionLog(sessionID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errorResp("日志不存在: "+err.Error()))
	}
	if events == nil {
		events = []ai.LogEvent{}
	}

	// Build summary from all events (before filtering)
	summary := buildLogSummary(events)

	// Filter by type
	typeFilter := c.QueryParam("type")
	if typeFilter != "" {
		allowed := make(map[string]bool)
		for _, t := range strings.Split(typeFilter, ",") {
			allowed[strings.TrimSpace(t)] = true
		}
		filtered := make([]ai.LogEvent, 0, len(events))
		for _, ev := range events {
			if allowed[ev.Type] {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}

	// Keyword search
	search := c.QueryParam("search")
	if search != "" {
		lower := strings.ToLower(search)
		filtered := make([]ai.LogEvent, 0, len(events))
		for _, ev := range events {
			if strings.Contains(strings.ToLower(ev.Content), lower) ||
				strings.Contains(strings.ToLower(ev.ToolName), lower) ||
				strings.Contains(strings.ToLower(ev.ToolResult), lower) ||
				strings.Contains(strings.ToLower(ev.Error), lower) {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}

	// Sort (default: desc = newest first)
	order := c.QueryParam("order")
	if order == "asc" {
		// already asc from JSONL order
	} else {
		// reverse for desc
		for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
			events[i], events[j] = events[j], events[i]
		}
	}

	total := len(events)

	// Pagination
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 2000 {
		limit = 2000
	}
	if offset < 0 {
		offset = 0
	}
	if offset < len(events) {
		end := offset + limit
		if end > len(events) {
			end = len(events)
		}
		events = events[offset:end]
	} else {
		events = []ai.LogEvent{}
	}

	return c.JSON(http.StatusOK, successResp(map[string]any{
		"events":   events,
		"total":    total,
		"filtered": len(events),
		"summary":  summary,
	}))
}

// GetChatErrors returns only error events for a session (newest first).
// GET /api/logs/chat/:id/errors
func (h *LogHandler) GetChatErrors(c echo.Context) error {
	sessionID := c.Param("id")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session ID"))
	}

	events, err := h.chatLogger.GetSessionLog(sessionID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errorResp("日志不存在: "+err.Error()))
	}

	var errors []ai.LogEvent
	for _, ev := range events {
		if ev.Type == "error" {
			errors = append(errors, ev)
		}
	}
	// newest first
	for i, j := 0, len(errors)-1; i < j; i, j = i+1, j-1 {
		errors[i], errors[j] = errors[j], errors[i]
	}
	if errors == nil {
		errors = []ai.LogEvent{}
	}

	return c.JSON(http.StatusOK, successResp(map[string]any{"errors": errors}))
}

func buildLogSummary(events []ai.LogEvent) map[string]any {
	typeCounts := make(map[string]int)
	var errorCount, toolCallCount int
	roundSet := make(map[int]bool)
	for _, ev := range events {
		typeCounts[ev.Type]++
		if ev.Type == "error" {
			errorCount++
		}
		if ev.Type == "tool_call" || ev.Type == "tool_start" {
			toolCallCount++
		}
		if ev.Round > 0 {
			roundSet[ev.Round] = true
		}
	}
	return map[string]any{
		"errors":     errorCount,
		"tool_calls": toolCallCount,
		"rounds":     len(roundSet),
		"types":      typeCounts,
	}
}

// TailChatLog streams log events for a session via SSE.
// GET /api/logs/chat/:id/tail
func (h *LogHandler) TailChatLog(c echo.Context) error {
	sessionID := c.Param("id")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session ID"))
	}

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)
	flusher, _ := c.Response().Writer.(http.Flusher)

	ctx := c.Request().Context()

	// Subscribe to live events
	ch := h.chatLogger.Subscribe(sessionID)
	if ch == nil {
		fmt.Fprintf(c.Response(), "event: error\ndata: {\"error\":\"too many subscribers\"}\n\n")
		if flusher != nil { flusher.Flush() }
		return nil
	}
	defer h.chatLogger.Unsubscribe(sessionID, ch)

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-ch:
			if !ok { return nil }
			data, _ := json.Marshal(ev)
			fmt.Fprintf(c.Response(), "event: log\ndata: %s\n\n", data)
			if flusher != nil { flusher.Flush() }
		}
	}
}

// GetChatLogText 返回会话日志的纯文本视图（仅人类可读行）。
// GET /api/logs/chat/:id/text
func (h *LogHandler) GetChatLogText(c echo.Context) error {
	sessionID := c.Param("id")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session ID"))
	}

	text, err := h.chatLogger.GetSessionLogText(sessionID)
	if err != nil {
		return c.JSON(http.StatusNotFound, errorResp("日志不存在: "+err.Error()))
	}

	return c.Blob(http.StatusOK, "text/plain; charset=utf-8", []byte(text))
}

// DownloadChatLog serves the raw .log file for download.
// GET /api/logs/chat/:id/download
func (h *LogHandler) DownloadChatLog(c echo.Context) error {
	sessionID := c.Param("id")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session ID"))
	}

	filePath := h.findSessionLogFile(sessionID)
	if filePath == "" {
		return c.JSON(http.StatusNotFound, errorResp("日志文件不存在"))
	}

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))
	return c.File(filePath)
}

// ── System Logs ────────────────────────────────────────────────────

// ListSystemLogs returns available system log files.
// GET /api/logs/system?order=desc   // desc (default): 最新日期在前
func (h *LogHandler) ListSystemLogs(c echo.Context) error {
	files := h.scanSystemLogs()

	order := c.QueryParam("order")
	if order == "asc" {
		// 正序：最早日期在前
		sort.Slice(files, func(i, j int) bool {
			return (files[i]["date"].(string)) < (files[j]["date"].(string))
		})
	} else {
		// 默认倒序：最新日期在前（scanSystemLogs 已按倒序排列，此处再次确保）
		sort.Slice(files, func(i, j int) bool {
			return (files[i]["date"].(string)) > (files[j]["date"].(string))
		})
	}

	return c.JSON(http.StatusOK, successResp(map[string]any{"files": files}))
}

// GetSystemLog returns system log content with pagination, tail, level filter, search, and order.
// GET /api/logs/system/:date?tail=200&offset=0&limit=500&level=ERROR,WARN&search=timeout&order=desc
func (h *LogHandler) GetSystemLog(c echo.Context) error {
	date := c.Param("date")
	filePath := h.findSystemLogFile(date)
	if filePath == "" {
		return c.JSON(http.StatusNotFound, errorResp("该日期没有系统日志"))
	}

	levelFilter := c.QueryParam("level")
	levelSet := make(map[string]bool)
	if levelFilter != "" {
		for _, l := range strings.Split(levelFilter, ",") {
			levelSet[strings.TrimSpace(strings.ToUpper(l))] = true
		}
	}

	search := c.QueryParam("search")
	order := c.QueryParam("order")
	// order 默认 desc（最新在前）

	tail := c.QueryParam("tail")
	if tail != "" {
		n, _ := strconv.Atoi(tail)
		if n > 0 {
			lines, _, _ := readAllLinesFiltered(filePath, levelSet, search)
			if order != "asc" {
				// desc: tail 取最后 N 行并反转（最新在前）
				if len(lines) > n {
					lines = lines[len(lines)-n:]
				}
				for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
					lines[i], lines[j] = lines[j], lines[i]
				}
			} else {
				// asc: tail 取最后 N 行（保持文件顺序，最早在前）
				if len(lines) > n {
					lines = lines[len(lines)-n:]
				}
			}
			return c.JSON(http.StatusOK, successResp(map[string]any{"lines": lines, "total": len(lines)}))
		}
	}

	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 500
	}

	lines, total, truncated, err := readLinesRangeFiltered(filePath, offset, limit, levelSet, search, order)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("读取日志失败: "+err.Error()))
	}

	return c.JSON(http.StatusOK, successResp(map[string]any{
		"lines": lines, "offset": offset, "limit": limit,
		"total": total, "has_more": offset+limit < total,
		"truncated": truncated,
	}))
}

// DeleteChatLog deletes a chat session log from disk.
// DELETE /api/logs/chat/:id
func (h *LogHandler) DeleteChatLog(c echo.Context) error {
	sessionID := c.Param("id")
	if sessionID == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少 session ID"))
	}

	if h.chatLogger == nil || !h.chatLogger.Enabled() {
		return c.JSON(http.StatusNotFound, errorResp("日志功能未启用"))
	}

	// Prevent deleting an active session
	if h.chatLogger.IsActive(sessionID) {
		return c.JSON(http.StatusConflict, APIResponse{Code: http.StatusConflict, Message: "会话仍在运行，无法删除"})
	}

	if err := h.chatLogger.RemoveSession(sessionID); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("删除失败: "+err.Error()))
	}

	return c.JSON(http.StatusOK, successResp(map[string]any{"deleted": sessionID}))
}

// DeleteSystemLog deletes a system log file for the given date.
// DELETE /api/logs/system/:date
func (h *LogHandler) DeleteSystemLog(c echo.Context) error {
	date := c.Param("date")
	if date == "" {
		return c.JSON(http.StatusBadRequest, errorResp("缺少日期"))
	}

	filePath := h.findSystemLogFile(date)
	if filePath == "" {
		return c.JSON(http.StatusNotFound, errorResp("该日期没有系统日志"))
	}

	// Get file info before deleting
	info, err := os.Stat(filePath)
	if err != nil {
		return c.JSON(http.StatusNotFound, errorResp("日志文件不存在"))
	}
	size := info.Size()

	if err := os.Remove(filePath); err != nil {
		return c.JSON(http.StatusInternalServerError, errorResp("删除失败: "+err.Error()))
	}

	// Clean up empty parent directories up to the log root
	removeEmptyParents(filePath, h.systemLogDir)

	return c.JSON(http.StatusOK, successResp(map[string]any{
		"deleted": date,
		"path":    filePath,
		"size":    size,
	}))
}

// DownloadSystemLog serves the raw system log file.
// GET /api/logs/system/:date/download
func (h *LogHandler) DownloadSystemLog(c echo.Context) error {
	date := c.Param("date")
	filePath := h.findSystemLogFile(date)
	if filePath == "" {
		return c.JSON(http.StatusNotFound, errorResp("该日期没有系统日志"))
	}

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"yunxi-home-%s.log\"", date))
	return c.File(filePath)
}

// ── Helpers ────────────────────────────────────────────────────────

func (h *LogHandler) findSessionLogFile(sessionID string) string {
	if h.chatLogger == nil {
		return ""
	}
	dir := h.chatLogger.Dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".log" {
			continue
		}
		sanitized := sanitizeForFilename(sessionID)
		if len(sanitized) > 32 { sanitized = sanitized[:32] }
		if strings.Contains(e.Name(), sanitized) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

func (h *LogHandler) scanSystemLogs() []map[string]any {
	if h.systemLogDir == "" {
		return nil
	}
	var result []map[string]any

	// Walk year/month/day directories
	filepath.Walk(h.systemLogDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".log") && !strings.Contains(info.Name(), "yunxi") {
			return nil
		}
		// Extract date from path: .../YYYY/MM/DD/file
		rel, _ := filepath.Rel(h.systemLogDir, path)
		date := strings.ReplaceAll(filepath.Dir(rel), string(filepath.Separator), "-")
		result = append(result, map[string]any{
			"date": date, "path": rel, "size": info.Size(),
		})
		return nil
	})

	// 按日期倒序排列（最新在前）
	sort.Slice(result, func(i, j int) bool {
		return result[i]["date"].(string) > result[j]["date"].(string)
	})

	return result
}

func (h *LogHandler) findSystemLogFile(date string) string {
	if h.systemLogDir == "" {
		return ""
	}
	// Try: <date>.log, yunxi-home.log, or YYYY/MM/DD/file
	candidates := []string{
		filepath.Join(h.systemLogDir, date+".log"),
		filepath.Join(h.systemLogDir, "yunxi-home.log"),
	}
	// Also try date as path segments: 2026/06/01 → 2026-06-01
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// Try glob for any log file containing the date
	pattern := filepath.Join(h.systemLogDir, "*"+date+"*.log")
	if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
		return matches[0]
	}
	// Walk for the date in directory structure
	filepath.Walk(h.systemLogDir, func(path string, info os.FileInfo, err error) error {
		if info != nil && !info.IsDir() && strings.Contains(path, strings.ReplaceAll(date, "-", string(filepath.Separator))) {
			candidates = append(candidates, path)
		}
		return nil
	})
	for _, c := range candidates[2:] {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func countRounds(filename, dir string) int {
	f, err := os.Open(filepath.Join(dir, filename))
	if err != nil {
		return 0
	}
	defer f.Close()
	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), `"type":"round_start"`) {
			count++
		}
	}
	return count
}

func readLastNLinesFiltered(path string, n int, levels map[string]bool, search string) []string {
	lines, _, _ := readAllLinesFiltered(path, levels, search)
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}

// readLinesRangeFiltered 流式读取 + 过滤 + 分页。
// order="desc" 时先反转再分页（最新行在前），offset 从开始计数（反转后的开始）。
func readLinesRangeFiltered(path string, offset, limit int, levels map[string]bool, search, order string) ([]string, int, bool, error) {
	lines, total, truncated := readAllLinesFiltered(path, levels, search)

	if order != "asc" {
		// desc: 反转，最新行在前
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}

	// Pagination
	if offset >= total {
		return []string{}, total, false, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return lines[offset:end], total, truncated, nil
}

// readAllLinesFiltered 流式读取日志文件，按 level + search 过滤。
// 最多返回 maxSystemLogLines 行，超出则截断并标记 truncated。
// 返回 (lines, total, truncated)。
func readAllLinesFiltered(path string, levels map[string]bool, search string) ([]string, int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, false
	}
	defer f.Close()

	var result []string
	truncated := false
	searchLower := strings.ToLower(search)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 单行最大 1MB
	for scanner.Scan() {
		line := scanner.Text()

		// Level 过滤
		if len(levels) > 0 && !matchesLevel(line, levels) {
			continue
		}

		// 关键词搜索
		if searchLower != "" && !strings.Contains(strings.ToLower(line), searchLower) {
			continue
		}

		if len(result) >= maxSystemLogLines {
			truncated = true
			break
		}
		result = append(result, line)
	}

	return result, len(result), truncated
}

func matchesLevel(line string, levels map[string]bool) bool {
	for _, prefix := range []string{"level=", "level:"} {
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		rest := line[idx+len(prefix):]
		end := strings.IndexAny(rest, " \t,")
		if end < 0 {
			end = len(rest)
		}
		level := strings.ToUpper(rest[:end])
		return levels[level]
	}
	return true // no level found, include by default
}

func readLastNLines(path string, n int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	return lines
}

func readLinesRange(path string, offset, limit int) ([]string, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var allLines []string
	scanner := bufio.NewScanner(f)
	// Use a larger buffer for long lines
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	total := len(allLines)
	if offset >= total {
		return []string{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return allLines[offset:end], total, nil
}

func sanitizeForFilename(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			b = append(b, c)
		} else {
			b = append(b, '_')
		}
	}
	return string(b)
}

// removeEmptyParents removes empty parent directories of filePath up to (but not
// including) rootDir. Used to clean up after deleting a system log file.
func removeEmptyParents(filePath, rootDir string) {
	dir := filepath.Dir(filePath)
	for dir != rootDir && strings.HasPrefix(dir, rootDir) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
