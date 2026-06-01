// Package errcode defines structured error codes and the AppError type for the entire system.
package errcode

import "net/http"

// Code is a machine-readable error code returned in API responses.
type Code string

const (
	// ── General ──
	ErrInternal     Code = "INTERNAL_ERROR"
	ErrTimeout      Code = "TIMEOUT"
	ErrBadRequest   Code = "BAD_REQUEST"
	ErrNotFound     Code = "NOT_FOUND"
	ErrConflict     Code = "CONFLICT"
	ErrRateLimit    Code = "RATE_LIMITED"
	ErrUnauthorized Code = "UNAUTHORIZED"
	ErrForbidden    Code = "FORBIDDEN"
	ErrTokenExpired Code = "TOKEN_EXPIRED"

	// ── AI ──
	ErrAINotReady      Code = "AI_NOT_READY"
	ErrAIProviderFail  Code = "AI_PROVIDER_FAIL"
	ErrAIModelNotFound Code = "AI_MODEL_NOT_FOUND"
	ErrAIStreamBroken  Code = "AI_STREAM_BROKEN"

	// ── File ──
	ErrFileTooLarge  Code = "FILE_TOO_LARGE"
	ErrFileNotFound  Code = "FILE_NOT_FOUND"
	ErrQuotaExceeded Code = "QUOTA_EXCEEDED"

	// ── DNS ──
	ErrDNSUpdateFail   Code = "DNS_UPDATE_FAIL"
	ErrDNSProviderFail Code = "DNS_PROVIDER_FAIL"

	// ── Config ──
	ErrConfigInvalid  Code = "CONFIG_INVALID"
	ErrConfigSaveFail Code = "CONFIG_SAVE_FAIL"
)

// meta maps each error code to its HTTP status and Chinese message.
type meta struct {
	HTTPStatus int
	Message    string
}

var codeMeta = map[Code]meta{
	ErrInternal:     {http.StatusInternalServerError, "服务器内部错误"},
	ErrTimeout:      {http.StatusGatewayTimeout, "请求超时，请稍后重试"},
	ErrBadRequest:   {http.StatusBadRequest, "请求参数无效"},
	ErrNotFound:     {http.StatusNotFound, "资源不存在"},
	ErrConflict:     {http.StatusConflict, "资源冲突，请刷新后重试"},
	ErrRateLimit:    {http.StatusTooManyRequests, "请求过于频繁，请稍后重试"},
	ErrUnauthorized: {http.StatusUnauthorized, "未授权访问，请先登录"},
	ErrForbidden:    {http.StatusForbidden, "无权限访问"},
	ErrTokenExpired: {http.StatusUnauthorized, "登录已过期，请重新登录"},

	ErrAINotReady:      {http.StatusServiceUnavailable, "AI 服务尚未配置，请在设置中启用"},
	ErrAIProviderFail:  {http.StatusBadGateway, "AI 服务暂时不可用，请稍后重试"},
	ErrAIModelNotFound: {http.StatusBadRequest, "模型不存在或不可用"},
	ErrAIStreamBroken:  {http.StatusBadGateway, "AI 连接中断，请重试"},

	ErrFileTooLarge:  {http.StatusRequestEntityTooLarge, "文件大小超出限制"},
	ErrFileNotFound:  {http.StatusNotFound, "文件不存在或已删除"},
	ErrQuotaExceeded: {http.StatusInsufficientStorage, "存储配额不足，请清理空间"},

	ErrDNSUpdateFail:   {http.StatusInternalServerError, "DNS 记录更新失败"},
	ErrDNSProviderFail: {http.StatusBadGateway, "DNS 服务商连接失败"},

	ErrConfigInvalid:  {http.StatusBadRequest, "配置校验失败"},
	ErrConfigSaveFail: {http.StatusInternalServerError, "配置保存失败"},
}

// HTTPStatus returns the HTTP status code for c.
func (c Code) HTTPStatus() int {
	if m, ok := codeMeta[c]; ok {
		return m.HTTPStatus
	}
	return http.StatusInternalServerError
}

// Message returns the Chinese message for c.
func (c Code) Message() string {
	if m, ok := codeMeta[c]; ok {
		return m.Message
	}
	return "未知错误"
}
