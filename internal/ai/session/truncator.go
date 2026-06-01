package session

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

const maxToolResultLen = 32000

// TruncateResult intelligently truncates tool results.
func TruncateResult(content string) string {
	return smartTruncate(content, maxToolResultLen)
}

func smartTruncate(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	slog.Info("结果截断", "原长度", len(content), "阈值", maxLen)

	// JSON 数组: 只保留前 100 项
	if trimmed := strings.TrimSpace(content); strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		var arr []any
		if json.Unmarshal([]byte(trimmed), &arr) == nil && len(arr) > 100 {
			kept := arr[:100]
			data, _ := json.Marshal(kept)
			result := string(data) + fmt.Sprintf("\n... (共 %d 项, 仅显示前 100)", len(arr))
			slog.Info("JSON数组截断", "总项数", len(arr), "保留", 100)
			return result
		}
	}

	// 多行日志: 去重连续重复行, 保留头尾
	lines := strings.Split(content, "\n")
	if len(lines) > 500 {
		deduped := []string{lines[0]}
		for i := 1; i < len(lines); i++ {
			if lines[i] != lines[i-1] {
				deduped = append(deduped, lines[i])
			}
		}
		if len(deduped) > 120 {
			header := strings.Join(deduped[:10], "\n")
			tail := strings.Join(deduped[len(deduped)-80:], "\n")
			result := header + fmt.Sprintf("\n... (跳过 %d 行重复/冗余) ...\n", len(lines)-90) + tail
			slog.Info("多行日志截断", "原行数", len(lines), "去重后", len(deduped))
			return result
		}
		result := strings.Join(deduped, "\n")
		slog.Info("多行去重", "原行数", len(lines), "去重后", len(deduped))
		return result
	}

	result := content[:maxLen] + fmt.Sprintf("\n... (截断, 原 %d 字符)", len(content))
	slog.Info("强制截断", "原长度", len(content), "截断至", maxLen)
	return result
}
