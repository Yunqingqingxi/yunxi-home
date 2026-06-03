package qqbot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/dto/message"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/token"

	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
)

var log = logger.ForComponent("qqbot")

// Config QQ Bot 配置
type Config struct {
	AppID       string
	AppSecret   string
	GroupID     string
	SandboxRoot string // 文件沙箱根目录
	SignSecret  string // 文件下载 URL 签名密钥（与服务器 /dl 端点一致）
}

// Handler 指令处理函数
type Handler func(ctx context.Context, args []string) string

// SkillRunner 技能执行接口（避免循环依赖 ai/skill 包）
type SkillRunner interface {
	// ListSkills 返回可用技能名和简介列表
	ListSkills() map[string]string
	// RunSkill 执行指定技能，返回结果文本
	RunSkill(ctx context.Context, name string) string
}

// AIService AI 对话服务接口（避免循环依赖）
type AIService interface {
		StreamChat(ctx context.Context, sessionID, userID, userMessage string) <-chan AIEvent
		InjectSystemMessage(sessionID, content string)
		ClearSession(sessionID string)
		CompactSession(sessionID string) string
		ReloadSkills() error
		ReloadMCP() error
		CreateSkill(ctx context.Context, description string) (string, error)
		GetMCPServer(ctx context.Context, query string) string
	}

// AIEvent AI 流式事件
type AIEvent struct {
	Type    string
	Content string
	Tool    string
}

// Bot QQ 机器人
type Bot struct {
	cfg         Config
	api         openapi.OpenAPI
	tokenSource oauth2.TokenSource
	handlers    map[string]Handler
	aiService   AIService
	skillRunner SkillRunner
	msgLimiter  *rate.Limiter
	mu          sync.RWMutex
	lastCmd        map[string]string
	cmdMu          sync.RWMutex
	botUser        *dto.User // 机器人自身信息
	filePromptSent map[string]bool
	promptMu       sync.Mutex
	online         bool                // WebSocket 连接状态
	statusMu       sync.RWMutex
	sessionMgr     botgo.SessionManager // 复用 session 以支持断线恢复
}

// BotInfo 机器人基础信息
type BotInfo struct {
	AppID    string `json:"app_id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Online   bool   `json:"online"`
}

func New(cfg Config) (*Bot, error) {
	creds := &token.QQBotCredentials{
		AppID:     cfg.AppID,
		AppSecret: cfg.AppSecret,
	}
	tokenSource := token.NewQQBotTokenSource(creds)

	if err := token.StartRefreshAccessToken(context.Background(), tokenSource); err != nil {
		return nil, fmt.Errorf("start token refresh failed: %w", err)
	}

	return &Bot{
		cfg:         cfg,
		api:         botgo.NewOpenAPI(cfg.AppID, tokenSource),
		tokenSource: tokenSource,
		sessionMgr:  botgo.NewSessionManager(),
		handlers:    make(map[string]Handler),
		msgLimiter:     rate.NewLimiter(rate.Every(2*time.Second), 3),
		lastCmd:        make(map[string]string),
		filePromptSent: make(map[string]bool),
	}, nil
}

func (b *Bot) AppID() string { return b.cfg.AppID }

func (b *Bot) FetchBotInfo(ctx context.Context) {
	user, err := b.api.Me(ctx)
	if err != nil {
		log.Warn("QQ Bot 获取自身信息失败", "app_id", b.cfg.AppID, "error", err)
		return
	}
	b.botUser = user
	log.Info("QQ Bot 信息已获取", "app_id", b.cfg.AppID, "username", user.Username)
}

func (b *Bot) GetBotInfo() *BotInfo {
	b.statusMu.RLock()
	online := b.online
	b.statusMu.RUnlock()
	if b.botUser == nil {
		return &BotInfo{AppID: b.cfg.AppID, Online: online}
	}
	return &BotInfo{
		AppID:    b.cfg.AppID,
		Username: b.botUser.Username,
		Avatar:   b.botUser.Avatar,
		Online:   online,
	}
}

// SetOnline sets the WebSocket connection status.
func (b *Bot) SetOnline(v bool) {
	b.statusMu.Lock()
	b.online = v
	b.statusMu.Unlock()
}

// IsOnline returns whether the WebSocket is currently connected.
func (b *Bot) IsOnline() bool {
	b.statusMu.RLock()
	defer b.statusMu.RUnlock()
	return b.online
}

func (b *Bot) RegisterCommand(cmd string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[cmd] = handler
}

// SetAIService 注入 AI 对话服务
func (b *Bot) SetAIService(svc AIService) {
	b.aiService = svc
}

// SetSkillRunner 注入技能执行器，并将技能注册为 /skillname 指令
func (b *Bot) SetSkillRunner(r SkillRunner) {
	b.skillRunner = r
	// 动态注册技能为 /-指令
	for name, desc := range r.ListSkills() {
		skillName := name
		_ = desc
		b.RegisterCommand("/"+skillName, func(ctx context.Context, args []string) string {
			return b.skillRunner.RunSkill(ctx, skillName)
		})
	}
}

func (b *Bot) hasSeenFilePrompt(userID string) bool {
	b.promptMu.Lock()
	defer b.promptMu.Unlock()
	return b.filePromptSent[userID]
}

func (b *Bot) markFilePromptSent(userID string) {
	b.promptMu.Lock()
	defer b.promptMu.Unlock()
	b.filePromptSent[userID] = true
}

func (b *Bot) Start(ctx context.Context) error {
	backoff := 1 * time.Second
	const maxBackoff = 60 * time.Second
	intent := dto.IntentGuilds | dto.IntentGroupMessages

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 每次重连重新获取 gateway（URL 可能变化）
		apInfo, err := b.api.WS(ctx, nil, "")
		if err != nil {
			log.Error("QQ Bot 获取 gateway 失败，稍后重试", "error", err, "backoff", backoff)
			select {
			case <-time.After(backoff):
				backoff = min(backoff*2, maxBackoff)
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		log.Info("QQ Bot gateway acquired", "url", apInfo.URL, "shards", apInfo.Shards)

		// 设置全局事件处理器
		event.DefaultHandlers.C2CMessage = func(ev *dto.WSPayload, data *dto.WSC2CMessageData) error {
			b.handlePrivateMessage(ctx, data)
			return nil
		}
		event.DefaultHandlers.GroupATMessage = func(ev *dto.WSPayload, data *dto.WSGroupATMessageData) error {
			b.handleGroupMessage(ctx, data)
			return nil
		}

		b.SetOnline(true)
		log.Info("QQ Bot WebSocket 连接中...")
		err = b.sessionMgr.Start(apInfo, b.tokenSource, &intent)
		b.SetOnline(false)

		if err != nil {
			log.Error("QQ Bot WebSocket 断开，将重连", "error", err, "backoff", backoff)
		} else {
			log.Info("QQ Bot WebSocket 正常关闭，将重连")
			backoff = 1 * time.Second // 正常关闭不加速退避
		}

		select {
		case <-time.After(backoff):
			backoff = min(backoff*2, maxBackoff)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (b *Bot) handleGroupMessage(ctx context.Context, data *dto.WSGroupATMessageData) {
	if data == nil || data.Author == nil || data.Author.Bot {
		return
	}
	content := strings.TrimSpace(message.ETLInput(data.Content))
	if content == "" {
		return
	}
	userID := data.Author.ID
	ctx = context.WithValue(ctx, "user_id", userID)
	ctx = context.WithValue(ctx, "is_group", true)

	response, _ := b.processMessage(ctx, userID, content)
	if response != "" {
		if !b.msgLimiter.Allow() {
			log.Debug("QQ Bot rate limited", "user", userID)
			return
		}
		b.replyMarkdown(ctx, data.GroupID, data.ID, response, true)
	}
}

func (b *Bot) handlePrivateMessage(ctx context.Context, data *dto.WSC2CMessageData) {
	if data == nil || data.Author == nil || data.Author.Bot {
		return
	}
	content := strings.TrimSpace(message.ETLInput(data.Content))
	userID := data.Author.ID
	ctx = context.WithValue(ctx, "user_id", userID)

	// 处理文件附件
	if len(data.Attachments) > 0 {
		fileRefs := b.downloadAttachments(ctx, data.Attachments)
		if len(fileRefs) > 0 {
			if content == "" {
				content = "请分析这些文件:"
			}
			content = strings.Join(fileRefs, "\n") + "\n" + content
		}
	}
	if content == "" {
		return
	}
	log.Debug("QQ Bot 收到单聊消息", "content", content, "user", userID, "attachments", len(data.Attachments))

	response, _ := b.processMessage(ctx, userID, content)
	if response != "" {
		if !b.msgLimiter.Allow() {
			return
		}
		b.replyMarkdown(ctx, "", userID, response, false)
	}
}

// processMessage 处理消息：指令优先，非指令走 AI 对话
func (b *Bot) processMessage(ctx context.Context, userID, content string) (string, bool) {
	cmd, args := parseCommand(content)

	if cmd != "" {
		b.cmdMu.Lock()
		b.lastCmd[userID] = cmd
		b.cmdMu.Unlock()

		b.mu.RLock()
		handler, ok := b.handlers[cmd]
		b.mu.RUnlock()
		if ok {
			return handler(ctx, args), false
		}
		return fmt.Sprintf("未知指令: %s\n发送 /help 查看可用指令", cmd), false
	}

	// 非指令消息 → AI 对话
	if b.aiService != nil {
		return b.handleAIChat(ctx, userID, content)
	}
	return "", false
}

// handleAIChat 通过 AI Service 处理对话并返回 Markdown 格式回复
func (b *Bot) handleAIChat(ctx context.Context, userID, message string) (string, bool) {
	sessionID := "qqbot_" + userID

	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	// 文件发送指令：通过系统消息注入，不污染用户可见的聊天记录
	if !b.hasSeenFilePrompt(userID) {
		b.aiService.InjectSystemMessage(sessionID,
			"[系统指令] 当你要发送文件给用户时，**必须**在回复中使用 `[文件: 文件名 (/path/to/file)]` 格式标记每个文件。"+
				"然后正常回复文字说明。这样文件才会被实际发送。")
		b.markFilePromptSent(userID)
	}

	stream := b.aiService.StreamChat(ctx, sessionID, "", message)

	var contentBuf strings.Builder

	for ev := range stream {
		switch ev.Type {
		case "thinking":
		case "content":
			contentBuf.WriteString(ev.Content)
		case "tool_result":
			// Reset buffer after each tool result — only keep the FINAL round's content.
			// Prevents multi-round accumulated intermediate text from being sent as duplicates.
			contentBuf.Reset()
		case "tool_call":
		case "error":
			return fmt.Sprintf("AI 服务异常: %s", ev.Content), false
		}
	}

	if contentBuf.Len() == 0 {
		return "已处理（无返回内容）", false
	}

	reply := contentBuf.String()
	// 提取并发送文件，返回纯文本（不含文件引用标记）
	reply = b.extractAndSendFiles(ctx, userID, reply)
	if reply == "" {
		reply = "文件已发送"
	}

	return reply, true
}

// sendC2CFile 上传并发送文件给 C2C 用户。
// 小文件（< 10MB）用 base64 JSON 上传，大文件用 multipart/form-data 流式上传。
func (b *Bot) sendC2CFile(ctx context.Context, userID, filePath, fileName string) error {
	// 获取文件信息
	st, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("读取文件信息失败: %w", err)
	}
	fileSize := st.Size()

	ext := strings.ToLower(filepath.Ext(fileName))
	fileType := 1 // 默认图片
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		fileType = 1
	case ".mp4", ".avi", ".mov", ".mkv":
		fileType = 2
	case ".mp3", ".wav", ".aac", ".silk":
		fileType = 3
	default:
		fileType = 4 // 普通文件
	}

	// Step 1: 上传文件
	var fileInfo []byte
	const largeFileThreshold = 10 * 1024 * 1024 // 10MB
	if fileSize < largeFileThreshold {
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("读取文件失败: %w", err)
		}
		fileInfo, err = b.uploadC2CFileBase64(ctx, userID, fileData, fileName, fileType)
		if err != nil {
			return fmt.Errorf("上传QQ文件失败: %w", err)
		}
	} else {
		var err error
		fileInfo, err = b.uploadC2CFileMultipart(ctx, userID, filePath, fileName, fileType, fileSize)
		if err != nil {
			return fmt.Errorf("上传QQ大文件失败: %w", err)
		}
	}

	// Step 2: 发送富媒体消息
	sendBody := fmt.Sprintf(`{"msg_type":7,"media":{"file_info":"%s"}}`, string(fileInfo))
	sendReq, _ := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://api.sgroup.qq.com/v2/users/%s/messages", userID),
		bytes.NewReader([]byte(sendBody)))
	sendReq.Header.Set("Content-Type", "application/json")
	sendReq.Header.Set("Authorization", "QQBot "+b.getAccessToken())

	sendClient := &http.Client{Timeout: 30 * time.Second}
	sendResp, err := sendClient.Do(sendReq)
	if err != nil {
		return fmt.Errorf("发送文件消息失败: %w", err)
	}
	defer sendResp.Body.Close()

	sendRespBody, _ := io.ReadAll(sendResp.Body)
	if sendResp.StatusCode != 200 && sendResp.StatusCode != 201 {
		return fmt.Errorf("发送文件消息失败 status=%d: %s", sendResp.StatusCode, string(sendRespBody))
	}
	log.Info("QQ Bot 文件已发送", "user", userID, "file", fileName, "size", fileSize)
	return nil
}

// uploadC2CFileBase64 小文件上传（base64 JSON body，适用于 < 10MB）。
func (b *Bot) uploadC2CFileBase64(ctx context.Context, userID string, data []byte, fileName string, fileType int) ([]byte, error) {
	payload := map[string]any{
		"file_type":    fileType,
		"url":          "",
		"srv_send_msg": false,
		"file_data":    base64.StdEncoding.EncodeToString(data),
	}
	body, _ := json.Marshal(payload)

	apiURL := fmt.Sprintf("https://api.sgroup.qq.com/v2/users/%s/files", userID)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "QQBot "+b.getAccessToken())

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上传请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Debug("QQ Bot 文件上传响应", "status", resp.StatusCode, "body", string(respBody)[:min(len(respBody), 300)])

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("上传失败 status=%d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		FileUUID string `json:"file_uuid"`
		FileInfo string `json:"file_info"`
	}
	json.Unmarshal(respBody, &result)
	if result.FileInfo != "" {
		return []byte(result.FileInfo), nil
	}
	if result.FileUUID != "" {
		return []byte(result.FileUUID), nil
	}
	return nil, fmt.Errorf("上传响应缺少 file_info: %s", string(respBody))
}

// uploadC2CFileMultipart 大文件流式上传（multipart/form-data，适用于 ≥ 10MB 的视频等）。
func (b *Bot) uploadC2CFileMultipart(ctx context.Context, userID, filePath, fileName string, fileType int, fileSize int64) ([]byte, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	// 异步写入 multipart body
	go func() {
		defer pw.Close()
		defer mw.Close()

		// file_type 字段
		mw.WriteField("file_type", fmt.Sprintf("%d", fileType))
		mw.WriteField("srv_send_msg", "false")

		// 文件流
		part, err := mw.CreateFormFile("file", fileName)
		if err != nil {
			log.Error("QQ Bot multipart: 创建文件字段失败", "error", err)
			return
		}
		written, err := io.Copy(part, f)
		if err != nil {
			log.Error("QQ Bot multipart: 写入文件流失败", "error", err, "written", written)
		}
	}()

	apiURL := fmt.Sprintf("https://api.sgroup.qq.com/v2/users/%s/files", userID)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "QQBot "+b.getAccessToken())

	// 大文件超时按文件大小估算（每 GB 约 5 分钟 + 2 分钟基础）
	timeout := 2*time.Minute + time.Duration(fileSize/(1024*1024*1024))*5*time.Minute
	if timeout > 30*time.Minute {
		timeout = 30 * time.Minute
	}
	client := &http.Client{Timeout: timeout}
	log.Info("QQ Bot 开始上传大文件", "file", fileName, "sizeMB", fileSize/(1024*1024), "timeout", timeout)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("上传请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("上传失败 status=%d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		FileUUID string `json:"file_uuid"`
		FileInfo string `json:"file_info"`
	}
	json.Unmarshal(respBody, &result)
	if result.FileInfo != "" {
		return []byte(result.FileInfo), nil
	}
	if result.FileUUID != "" {
		return []byte(result.FileUUID), nil
	}
	return nil, fmt.Errorf("上传响应缺少 file_info")
}

// getAccessToken 获取当前 access token
func (b *Bot) getAccessToken() string {
	tok, err := b.tokenSource.Token()
	if err != nil {
		return ""
	}
	return tok.AccessToken
}

var fileRefRe = regexp.MustCompile(`\[文件:\s*([^\]]+?)\s*\(([^)]+)\)\]`)

// extractAndSendFiles 从回复文本中提取文件引用并发送给用户
func (b *Bot) extractAndSendFiles(ctx context.Context, userID, text string) string {
	// 群聊暂不支持文件发送
	if isGroup, _ := ctx.Value("is_group").(bool); isGroup {
		return fileRefRe.ReplaceAllString(text, "")
	}
	matches := fileRefRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return text
	}
	for _, m := range matches {
	name := m[1]
	p := m[2]
	// 自动补全扩展名：如果显示名没有后缀，从路径中提取
	if filepath.Ext(name) == "" {
		if ext := filepath.Ext(p); ext != "" {
			name = name + ext
		}
	}
	// 沙箱路径 → 真实文件系统路径（始终拼 SandboxRoot）
	fullPath := filepath.Join(b.cfg.SandboxRoot, strings.TrimPrefix(p, "/")) // TrimPrefix 防止绝对路径覆盖
	if _, err := os.Stat(fullPath); err == nil {
	 log.Debug("QQ Bot 检测到文件引用，准备发送", "name", name, "path", fullPath)
	 if err := b.sendC2CFile(ctx, userID, fullPath, name); err != nil {
	 log.Warn("QQ Bot 发送文件失败", "name", name, "error", err)
	}
	} else {
	log.Warn("QQ Bot 文件不存在，跳过发送", "name", name, "path", fullPath, "error", err)
	}
	}
	// 去掉文件引用标记，保留纯文本
	result := fileRefRe.ReplaceAllString(text, "")
	// 清理残留：空反引号对、空列表项、多余空行
	bt := "`"
	result = regexp.MustCompile(bt+`\s*`+bt).ReplaceAllString(result, "")
	result = regexp.MustCompile(`(?m)^[\s]*[-*+]\s*$`).ReplaceAllString(result, "")
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")
	return strings.TrimSpace(result)
}

// QQMsgMaxLen 单条 Markdown 消息安全长度上限
const QQMsgMaxLen = 3500

// chunkMarkdown 智能分片：在自然边界处断开，保护 Markdown 语法完整性。
// 优先级: 双换行 → 单换行 → 句子结束符 → 逗号分号 → 强制截断
func chunkMarkdown(text string) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= QQMsgMaxLen {
		return []string{string(runes)}
	}

	var chunks []string
	remaining := string(runes)

	for len([]rune(remaining)) > QQMsgMaxLen {
		slice := []rune(remaining)
		cut := findCutPoint(slice, QQMsgMaxLen)
		chunks = append(chunks, strings.TrimSpace(string(slice[:cut])))
		remaining = strings.TrimSpace(string(slice[cut:]))
	}
	if len([]rune(remaining)) > 0 {
		chunks = append(chunks, remaining)
	}
	if len(chunks) > 3 {
		chunks = chunks[:3]
		chunks[2] += "\n\n> ...(后续内容已省略)"
	}
	return chunks
}

func findCutPoint(runes []rune, maxLen int) int {
	searchEnd := maxLen
	if searchEnd > len(runes) {
		searchEnd = len(runes)
	}
	window := string(runes[:searchEnd])

	// 1. 双换行（段落边界）
	if idx := strings.LastIndex(window, "\n\n"); idx > maxLen/2 {
		return idx + 2
	}
	// 2. 单换行
	if idx := strings.LastIndex(window, "\n"); idx > maxLen/2 {
		return idx + 1
	}
	// 3. 中文句号
	if idx := strings.LastIndex(window, "。"); idx > maxLen*2/3 {
		return idx + len("。")
	}
	// 4. 英文句号+空格
	if idx := strings.LastIndex(window, ". "); idx > maxLen*2/3 {
		return idx + 2
	}
	// 5. 问号/感叹号
	for _, sep := range []string{"？", "！", "? ", "! "} {
		if idx := strings.LastIndex(window, sep); idx > maxLen*3/4 {
			return idx + len(sep)
		}
	}
	// 6. 逗号/分号
	for _, sep := range []string{"，", "；", ", ", "; "} {
		if idx := strings.LastIndex(window, sep); idx > maxLen*3/4 {
			return idx + len(sep)
		}
	}
	// 7. 保底：强制截断
	return maxLen
}

// replyMarkdown 以 Markdown 格式回复（支持智能分片 + 降级纯文本）。
// isGroup: true=群聊(需msgID), false=单聊(需userID)
func (b *Bot) replyMarkdown(ctx context.Context, groupID, msgID, text string, isGroup bool) {
	chunks := chunkMarkdown(text)
	for i, chunk := range chunks {
		if i > 0 && !b.msgLimiter.Allow() {
			log.Debug("QQ Bot rate limited during chunk send", "chunk", i)
			return
		}
		var err error
		if isGroup {
			err = b.sendGroupMarkdownChunk(ctx, groupID, msgID, chunk)
		} else {
			err = b.sendPrivateMarkdownChunk(ctx, msgID, chunk)
		}
		// Markdown 发送失败时降级为纯文本
		if err != nil {
			log.Debug("Markdown发送失败，降级为纯文本", "error", err, "chunk", i)
			if isGroup {
				b.replyGroup(ctx, groupID, msgID, chunk, false)
			} else {
				b.replyPrivate(ctx, msgID, chunk, false)
			}
		}
	}
}

func (b *Bot) sendGroupMarkdownChunk(ctx context.Context, groupID, msgID, text string) error {
	msg := &dto.MessageToCreate{
		MsgID:    msgID,
		MsgType:  2,
		Markdown: &dto.Markdown{Content: text},
	}
	_, err := b.api.PostGroupMessage(ctx, groupID, msg)
	return err
}

func (b *Bot) sendPrivateMarkdownChunk(ctx context.Context, userID, text string) error {
	msg := &dto.MessageToCreate{
		MsgType:  2,
		Markdown: &dto.Markdown{Content: text},
	}
	_, err := b.api.PostC2CMessage(ctx, userID, msg)
	return err
}

// stripMarkdown 移除所有 Markdown 语法，返回纯文本
func stripMarkdown(text string) string {
	re := regexp.MustCompile("(?s)```[^`]*```")
	text = re.ReplaceAllString(text, "")
	re = regexp.MustCompile("`([^`]+)`")
	text = re.ReplaceAllString(text, "$1")
	re = regexp.MustCompile("(?m)^\\|.*\\|$")
	text = re.ReplaceAllString(text, "")
	text = strings.NewReplacer("**", "", "*", "", "__", "", "~~", "").Replace(text)
	re = regexp.MustCompile("(?m)^#{1,6} ")
	text = re.ReplaceAllString(text, "")
	re = regexp.MustCompile("(?m)^[-*]{3,}$")
	text = re.ReplaceAllString(text, "")
	re = regexp.MustCompile("(?m)^> ")
	text = re.ReplaceAllString(text, "")
	re = regexp.MustCompile("\n{3,}")
	text = re.ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// truncateForQQ 截断文本到 QQ Markdown 消息安全长度上限
func truncateForQQ(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= QQMsgMaxLen {
		return string(runes)
	}
	return string(runes[:QQMsgMaxLen])
}

func (b *Bot) replyGroup(ctx context.Context, groupID, msgID, text string, markdown bool) {
	msg := &dto.MessageToCreate{
		MsgID: msgID,
		MessageReference: &dto.MessageReference{
			MessageID:             msgID,
			IgnoreGetMessageError: true,
		},
	}
	if markdown {
		msg.MsgType = 2
		msg.Markdown = &dto.Markdown{Content: text}
	} else {
		msg.MsgType = 0
		msg.Content = text
	}
	if _, err := b.api.PostGroupMessage(ctx, groupID, msg); err != nil {
		log.Error("group reply failed", "error", err)
	}
}

func (b *Bot) replyPrivate(ctx context.Context, userID, text string, markdown bool) {
	msg := &dto.MessageToCreate{}
	if markdown {
		msg.MsgType = 2
		msg.Markdown = &dto.Markdown{Content: text}
	} else {
		msg.MsgType = 0
		msg.Content = text
	}
	if _, err := b.api.PostC2CMessage(ctx, userID, msg); err != nil {
		log.Error("private reply failed", "error", err)
	}
}

func (b *Bot) SendGroupMessage(ctx context.Context, content string) error {
	if b.cfg.GroupID == "" {
		return fmt.Errorf("group ID not configured")
	}
	msg := &dto.MessageToCreate{Content: content}
	_, err := b.api.PostGroupMessage(ctx, b.cfg.GroupID, msg)
	return err
}

// SendGroupMarkdown 发送 Markdown 格式的群消息（通知用，不分片）
func (b *Bot) SendGroupMarkdown(ctx context.Context, markdownText string) error {
	if b.cfg.GroupID == "" {
		return fmt.Errorf("group ID not configured")
	}
	// 通知类消息通常不长，做一次智能分片保底
	chunks := chunkMarkdown(markdownText)
	for _, chunk := range chunks {
		msg := &dto.MessageToCreate{
		MsgType:  2,
		Markdown: &dto.Markdown{Content: chunk},
		}
		if _, err := b.api.PostGroupMessage(ctx, b.cfg.GroupID, msg); err != nil {
			// 降级纯文本
			msg.MsgType = 0
			msg.Content = chunk
			_, err = b.api.PostGroupMessage(ctx, b.cfg.GroupID, msg)
			return err
		}
	}
	return nil
}

// downloadAttachments 下载 QQ 消息中的附件到沙箱
func (b *Bot) downloadAttachments(ctx context.Context, attachments []*dto.MessageAttachment) []string {
	sandboxRoot := b.cfg.SandboxRoot
	if sandboxRoot == "" {
		if runtime.GOOS == "windows" {
			home, _ := os.UserHomeDir()
			sandboxRoot = filepath.Join(home, ".yunxi", "data", "yunxiFiles")
		} else {
			sandboxRoot = "/opt/yunxi-home/data/yunxiFiles"
		}
	}
	botDir := filepath.Join(sandboxRoot, "qqbot")
	os.MkdirAll(botDir, 0755)

	var refs []string
	client := &http.Client{Timeout: 30 * time.Second}

	for _, att := range attachments {
		if att.URL == "" {
			continue
		}
		fname := att.FileName
		if fname == "" {
			fname = fmt.Sprintf("file_%d", time.Now().UnixNano())
		}
		destPath := filepath.Join(botDir, fname)

		req, err := http.NewRequestWithContext(ctx, "GET", att.URL, nil)
		if err != nil {
			log.Warn("QQ Bot 附件下载失败", "url", att.URL, "error", err)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Warn("QQ Bot 附件下载失败", "url", att.URL, "error", err)
			continue
		}
		f, err := os.Create(destPath)
		if err != nil {
			resp.Body.Close()
			log.Warn("QQ Bot 附件保存失败", "path", destPath, "error", err)
			continue
		}
		_, err = io.Copy(f, resp.Body)
		resp.Body.Close()
		f.Close()
		if err != nil {
			log.Warn("QQ Bot 附件写入失败", "path", destPath, "error", err)
			continue
		}
		log.Debug("QQ Bot 附件已下载", "name", fname, "size", att.Size, "path", "/qqbot/"+fname)
		// Use the format the AI understands: [文件: name (path)]
		refs = append(refs, fmt.Sprintf("[文件: %s (/qqbot/%s)]", fname, fname))
	}
	return refs
}

// ── Rich Media (msg_type=7) ──────────────────────────────────

// mediaUploadResp QQ 文件上传 API 返回结构
type mediaUploadResp struct {
	FileUUID string `json:"file_uuid"`
	FileInfo string `json:"file_info"`
	TTL      int    `json:"ttl"`
	ID       string `json:"id,omitempty"`
}

// uploadMedia 上传文件到 QQ 平台，返回 file_info。
// fileType: 1=图片, 2=视频, 3=语音, 4=文件
// fileURL: 文件的公网可访问 URL（QQ 服务器从此地址拉取）
func (b *Bot) uploadMedia(ctx context.Context, openid, fileURL string, fileType int, isGroup bool) (*mediaUploadResp, error) {
	var url string
	if isGroup {
		url = fmt.Sprintf("https://api.sgroup.qq.com/v2/groups/%s/files", openid)
	} else {
		url = fmt.Sprintf("https://api.sgroup.qq.com/v2/users/%s/files", openid)
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	w.WriteField("file_type", fmt.Sprintf("%d", fileType))
	w.WriteField("url", fileURL)
	w.WriteField("srv_send_msg", "false")
	w.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", url, &body)
	if err != nil {
		return nil, fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	tok, err := b.tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}
	req.Header.Set("Authorization", "QQBot "+tok.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload media HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result mediaUploadResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode upload response: %w", err)
	}
	log.Debug("QQ media uploaded", "file_uuid", result.FileUUID, "ttl", result.TTL)
	return &result, nil
}

// sendMedia 发送富媒体消息 (msg_type=7)
func (b *Bot) sendMedia(ctx context.Context, targetID, fileInfo string, isGroup bool) error {
	msg := &dto.MessageToCreate{
		MsgType: 7,
		Media:   &dto.MediaInfo{FileInfo: []byte(fileInfo)},
	}
	var err error
	if isGroup {
		_, err = b.api.PostGroupMessage(ctx, targetID, msg)
	} else {
		_, err = b.api.PostC2CMessage(ctx, targetID, msg)
	}
	return err
}

// replyMedia 上传并发送富媒体回复（失败时降级文本通知，附带错误原因）
func (b *Bot) replyMedia(ctx context.Context, targetID, msgID, fileURL string, fileType int, fallbackText string, isGroup bool) {
	uploadResp, err := b.uploadMedia(ctx, targetID, fileURL, fileType, isGroup)
	if err != nil {
		log.Warn("media upload failed, fallback to text", "error", err)
		msg := fallbackText + "\n\n> 上传失败: " + err.Error()
		if isGroup {
			b.replyGroup(ctx, targetID, msgID, msg, false)
		} else {
			b.replyPrivate(ctx, msgID, msg, false)
		}
		return
	}

	if err := b.sendMedia(ctx, targetID, uploadResp.FileInfo, isGroup); err != nil {
		log.Warn("media send failed, fallback to text", "error", err)
		msg := fallbackText + "\n\n> 发送失败: " + err.Error()
		if isGroup {
			b.replyGroup(ctx, targetID, msgID, msg, false)
		} else {
			b.replyPrivate(ctx, msgID, msg, false)
		}
	}
}

func parseCommand(content string) (string, []string) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "/") {
		return "", nil
	}
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return "", nil
	}
	return strings.ToLower(parts[0]), parts[1:]
}
