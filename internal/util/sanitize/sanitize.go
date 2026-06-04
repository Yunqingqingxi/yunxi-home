// Package sanitize provides input validation, sanitization, and boundary-checking
// utilities to protect against user misoperation, malformed data, and injection.
package sanitize

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Common limits
const (
	MaxStringLen    = 64 * 1024  // 64KB for general text fields
	MaxShortString  = 1024       // 1KB for names, titles
	MaxTinyString   = 256        // 256 bytes for IDs, keys
	MaxURL          = 8 * 1024   // URL max length
	MaxPath         = 4096       // File path max length
	MaxJSON         = 1024 * 1024 // 1MB for JSON payloads
)

// ── String validation ──────────────────────────────────────────────────

// String validates a string is within length bounds and contains valid UTF-8.
// Returns the trimmed string and nil, or empty and an error.
func String(s string, maxLen int) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("值不能为空")
	}
	if len(s) > maxLen {
		return "", fmt.Errorf("值过长 (最大 %d 字符)", maxLen)
	}
	if !utf8.ValidString(s) {
		return "", fmt.Errorf("包含无效字符")
	}
	return s, nil
}

// OptionalString validates a string if non-empty, returns empty if blank.
func OptionalString(s string, maxLen int) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	return String(s, maxLen)
}

// ── Path validation ────────────────────────────────────────────────────

// PathSanitize cleans and validates a file path, rejecting traversal attempts.
func PathSanitize(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/", nil
	}
	// Reject explicit traversal
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("路径不能包含 '..'")
	}
	// Reject null bytes (path injection)
	if strings.Contains(p, "\x00") {
		return "", fmt.Errorf("路径包含无效字符")
	}
	// Clean the path
	cleaned := filepath.Clean(p)
	if len(cleaned) > MaxPath {
		return "", fmt.Errorf("路径过长 (最大 %d 字符)", MaxPath)
	}
	return cleaned, nil
}

// ── Hostname / IP validation ───────────────────────────────────────────

// HostPort validates a "host:port" string.
func HostPort(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("地址不能为空")
	}
	host, port, err := net.SplitHostPort(s)
	if err != nil {
		// Try parsing as host without port (DNS operations often use endpoint only)
		if strings.Contains(s, ":") {
			return "", fmt.Errorf("地址格式无效: %w", err)
		}
		return s, nil
	}
	if host == "" {
		return "", fmt.Errorf("主机名不能为空")
	}
	if port == "" {
		return "", fmt.Errorf("端口不能为空")
	}
	return net.JoinHostPort(host, port), nil
}

// ── URL validation ─────────────────────────────────────────────────────

// URL validates a URL string.
func URL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("URL 不能为空")
	}
	if len(s) > MaxURL {
		return "", fmt.Errorf("URL 过长 (最大 %d 字符)", MaxURL)
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("URL 格式无效: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("仅支持 http/https 协议")
	}
	return s, nil
}

// ── Domain name validation ─────────────────────────────────────────────

var domainRe = regexp.MustCompile(`^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// Domain validates a DNS domain name (supports wildcard *.example.com).
func Domain(s string) (string, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "", fmt.Errorf("域名不能为空")
	}
	if len(s) > 253 {
		return "", fmt.Errorf("域名过长 (最大 253 字符)")
	}
	if !domainRe.MatchString(s) {
		return "", fmt.Errorf("域名格式无效: %s", s)
	}
	return s, nil
}

// ── Record type validation ─────────────────────────────────────────────

var validRecordTypes = map[string]bool{
	"A": true, "AAAA": true, "CNAME": true, "MX": true, "TXT": true,
	"NS": true, "SRV": true, "CAA": true, "PTR": true,
}

// RecordType validates a DNS record type.
func RecordType(s string) (string, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if !validRecordTypes[s] {
		return "", fmt.Errorf("不支持的记录类型: %s", s)
	}
	return s, nil
}

// ── Email validation ───────────────────────────────────────────────────

var emailRe = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Email validates an email address.
func Email(s string) (string, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "", fmt.Errorf("邮箱不能为空")
	}
	if len(s) > 254 {
		return "", fmt.Errorf("邮箱过长 (最大 254 字符)")
	}
	if !emailRe.MatchString(s) {
		return "", fmt.Errorf("邮箱格式无效: %s", s)
	}
	return s, nil
}

// ── Numeric validation ─────────────────────────────────────────────────

// PositiveInt validates a value is a positive integer within bounds.
func PositiveInt(v int, maxValue int) error {
	if v <= 0 {
		return fmt.Errorf("值必须为正整数")
	}
	if v > maxValue {
		return fmt.Errorf("值过大 (最大 %d)", maxValue)
	}
	return nil
}

// Port validates a TCP/UDP port number.
func Port(v int) error {
	if v < 1 || v > 65535 {
		return fmt.Errorf("端口号无效 (1-65535): %d", v)
	}
	return nil
}

// ── JSON field sanitization ────────────────────────────────────────────

// JSON sanitizes common injection vectors in JSON field values.
func JSON(s string) string {
	// Replace characters commonly used in injection attacks
	s = strings.ReplaceAll(s, "\x00", "")           // null bytes
	s = strings.ReplaceAll(s, "$(", "\\$(")         // shell substitution
	s = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return -1 // drop control chars except common whitespace
		}
		return r
	}, s)
	return s
}

// ── Safe defaults ──────────────────────────────────────────────────────

// Coalesce returns the first non-empty string, or the default.
func Coalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// Clamp returns v bounded to [min, max].
func Clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ── Duplicate detection ────────────────────────────────────────────────

// IsDuplicate returns true if the value already exists in the slice.
func IsDuplicate[T comparable](items []T, item T) bool {
	for _, v := range items {
		if v == item {
			return true
		}
	}
	return false
}

// Dedupe returns a deduplicated copy of the slice preserving order.
func Dedupe[T comparable](items []T) []T {
	seen := make(map[T]bool, len(items))
	result := make([]T, 0, len(items))
	for _, v := range items {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
