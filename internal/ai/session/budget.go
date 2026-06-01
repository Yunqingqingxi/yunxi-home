package session

import (
	"strings"
	"unicode/utf8"
)

// BudgetManager token 预算管理器
type BudgetManager struct {
	maxTokens       int
	reserveForReply int
	reserveForTools int
}

// NewBudgetManager 创建预算管理器
func NewBudgetManager(maxTokens, reserveForReply, reserveForTools int) *BudgetManager {
	if maxTokens <= 0 {
		maxTokens = 128000
	}
	if reserveForReply <= 0 {
		reserveForReply = 4096
	}
	if reserveForTools <= 0 {
		reserveForTools = 16384
	}
	return &BudgetManager{
		maxTokens:       maxTokens,
		reserveForReply: reserveForReply,
		reserveForTools: reserveForTools,
	}
}

// MessageWithContent 可估算 token 的消息
type MessageWithContent struct {
	Role    string
	Content string
}

// EstimateTokens 粗略估算 token 数
func (b *BudgetManager) EstimateTokens(messages []MessageWithContent) int {
	total := 0
	for _, msg := range messages {
		total += b.estimateString(msg.Role) + b.estimateString(msg.Content) + 4
	}
	return total
}

func (b *BudgetManager) estimateString(s string) int {
	chars := utf8.RuneCountInString(s)
	cjkChars := 0
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) || (r >= 0x20000 && r <= 0x2A6DF) {
			cjkChars++
		}
	}
	asciiChars := chars - cjkChars
	return int(float64(asciiChars)*0.25 + float64(cjkChars)*0.6)
}

// Available 返回可用的 token 数
func (b *BudgetManager) Available(messages []MessageWithContent) int {
	used := b.EstimateTokens(messages)
	available := b.maxTokens - used - b.reserveForReply - b.reserveForTools
	if available < 0 {
		return 0
	}
	return available
}

// NeedsCompact 是否需要压缩
func (b *BudgetManager) NeedsCompact(messages []MessageWithContent) bool {
	return b.Available(messages) < b.reserveForTools
}

// CompactHistory 压缩历史（缓存友好版）。
// 规则：
//  1. 永远保留 index 0 (system prompt) 不动 → 保护硬盘缓存前缀
//  2. 如果 message[1] 是 system 扩展消息（如 goal resume），也保留
//  3. 只压缩中间的消息，尾部保留最近 6 条
func (b *BudgetManager) CompactHistory(messages []MessageWithContent) []MessageWithContent {
	if len(messages) <= 7 {
		return messages
	}
	used := b.EstimateTokens(messages)
	if used < b.maxTokens-b.reserveForTools {
		return messages
	}

	// 缓存前缀保护：前 2 条消息不动（system + 可能的 system 扩展）
	prefixLen := 2
	if len(messages) > 2 && messages[1].Role == "system" {
		prefixLen = 2
	}
	prefix := messages[:prefixLen]            // 永远不动的缓存前缀
	recent := messages[len(messages)-6:]      // 尾部最近 6 条

	midStart := prefixLen
	midEnd := len(messages) - 6
	if midStart >= midEnd {
		return messages // 没有可压缩的中间部分
	}

	summary := MessageWithContent{
		Role:    "system",
		Content: buildCompactSummary(midEnd-midStart, messages[midStart:midEnd]),
	}

	result := make([]MessageWithContent, 0, prefixLen+1+6)
	result = append(result, prefix...)
	result = append(result, summary)
	result = append(result, recent...)
	return result
}

func buildCompactSummary(count int, old []MessageWithContent) string {
	sb := strings.Builder{}
	sb.WriteString("[对话历史摘要] 之前 ")
	sb.WriteString(itoa(count))
	sb.WriteString(" 条消息的关键信息:\n")

	keyResults := make([]string, 0, 5)
	for _, msg := range old {
		if len(msg.Content) > 10 {
			short := msg.Content
			if len(short) > 150 {
				short = short[:150] + "..."
			}
			keyResults = append(keyResults, msg.Role+": "+short)
		}
	}
	if len(keyResults) > 0 {
		for i, r := range keyResults {
			if i >= 5 {
				break
			}
			sb.WriteString("- ")
			sb.WriteString(r)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
