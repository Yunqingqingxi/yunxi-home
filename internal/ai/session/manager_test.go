package session

import (
	"testing"

	"github.com/Yunqingqingxi/yunxi-home/internal/ai/base"
	"github.com/Yunqingqingxi/yunxi-home/internal/models"
)

func TestGetHistorySkipsToolResultBlocks(t *testing.T) {
	m := NewManager(nil)
	// Manually create a session with blocks including both tool_call and tool_result
	st := &state{
		info:    newChatSession("test_skip_tool_result"),
		history: makeTestHistory(),
	}
	m.sessions["test_skip_tool_result"] = st

	detail, err := m.GetHistory("test_skip_tool_result")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	// Find assistant message with blocks
	for _, msg := range detail.Messages {
		if msg.Role != "assistant" {
			continue
		}
		// Should have exactly 3 blocks: thinking, content, tool_call (tool_result skipped)
		if len(msg.Blocks) == 0 {
			continue
		}
		// Count tool blocks
		toolCount := 0
		for _, b := range msg.Blocks {
			if b.Type == "tool_call" || b.Type == "tool_result" ||
				b.Type == string(base.BlockTypeToolCall) || b.Type == string(base.BlockTypeToolResult) {
				toolCount++
			}
		}
		// Should have exactly 1 tool block (tool_call with merged result), not 2
		if toolCount > 1 {
			t.Errorf("expected at most 1 tool block (tool_call with merged result), got %d", toolCount)
			for i, b := range msg.Blocks {
				t.Logf("  block[%d]: type=%s name=%s result=%s", i, b.Type, b.ToolName, b.ToolResult)
			}
		}
	}
}

func makeTestHistory() []base.Message {
	return []base.Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "test message"},
		{Role: "assistant", Content: "I'll do that", Blocks: []base.ContentBlock{
			{Type: base.BlockTypeThinking, Content: "thinking..."},
			{Type: base.BlockTypeContent, Content: "I'll do that"},
			{Type: base.BlockTypeToolCall, ToolName: "file_read", ToolArgs: `{"path":"/test"}`, ToolCallID: "call_1"},
			{Type: base.BlockTypeToolResult, ToolName: "file_read", ToolResult: "file contents here", ToolCallID: "call_1"},
		}},
		{Role: "tool", Content: "file contents here", ToolCallID: "call_1"},
	}
}

func newChatSession(id string) models.ChatSession {
	return models.ChatSession{ID: id}
}
