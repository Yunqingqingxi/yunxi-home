package collab

import (
	"fmt"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"sync"
	"time"
)

var log = logger.ForComponent("collab")

// Subscription 消息订阅（消费者接口）
type Subscription struct {
	AgentID string
	Topics  []string
	Ch      chan CollabMessage
}

// MessageBus in-process 协作消息总线
type MessageBus struct {
	mu            sync.RWMutex
	subscriptions map[string]*Subscription // key = agentID
	topicIndex    map[string][]string      // topic → agentIDs（加速路由）
	history       []CollabMessage          // 最近消息历史（用于延迟订阅者回放）
	maxHistory    int
	nextMsgID     int64
}

// NewMessageBus 创建消息总线
func NewMessageBus() *MessageBus {
	return &MessageBus{
		subscriptions: make(map[string]*Subscription),
		topicIndex:    make(map[string][]string),
		history:       make([]CollabMessage, 0, 256),
		maxHistory:    500,
	}
}

// Subscribe 订阅指定主题
func (b *MessageBus) Subscribe(agentID string, topics []string, bufSize int) *Subscription {
	if bufSize <= 0 {
		bufSize = 64
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// 如果已存在，先取消
	if old, ok := b.subscriptions[agentID]; ok {
		close(old.Ch)
		b.removeFromIndex(agentID, old.Topics)
	}

	sub := &Subscription{
		AgentID: agentID,
		Topics:  topics,
		Ch:      make(chan CollabMessage, bufSize),
	}
	b.subscriptions[agentID] = sub

	// 更新主题索引
	for _, topic := range topics {
		b.topicIndex[topic] = append(b.topicIndex[topic], agentID)
	}

	// 回放历史消息（匹配主题的）
	go b.replayHistory(sub)

	log.Debug("collab: subscribed", "agent", agentID, "topics", topics)
	return sub
}

// Unsubscribe 取消订阅
func (b *MessageBus) Unsubscribe(agentID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if sub, ok := b.subscriptions[agentID]; ok {
		close(sub.Ch)
		b.removeFromIndex(agentID, sub.Topics)
		delete(b.subscriptions, agentID)
	}
}

// Publish 发布消息到指定主题
func (b *MessageBus) Publish(topic string, fromAgent string, payload []byte, toAgent string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	msg := CollabMessage{
		ID:        b.genMsgID(),
		Topic:     topic,
		FromAgent: fromAgent,
		ToAgent:   toAgent,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	// 记录历史
	b.history = append(b.history, msg)
	if len(b.history) > b.maxHistory {
		b.history = b.history[len(b.history)-b.maxHistory:]
	}

	// 点对点消息
	if toAgent != "" {
		if sub, ok := b.subscriptions[toAgent]; ok {
			b.sendTo(sub, msg)
		}
		return
	}

	// 广播到订阅该主题的所有 Agent
	agentIDs, ok := b.topicIndex[topic]
	if !ok {
		return
	}
	for _, agentID := range agentIDs {
		if agentID == fromAgent {
			continue // 不发给自己
		}
		if sub, ok := b.subscriptions[agentID]; ok {
			b.sendTo(sub, msg)
		}
	}
}

// sendTo 非阻塞发送（panic-safe：频道可能已被 Close 关闭）
func (b *MessageBus) sendTo(sub *Subscription, msg CollabMessage) {
	defer func() {
		if r := recover(); r != nil {
			log.Debug("collab: sendTo recovered from panic", "agent", sub.AgentID)
		}
	}()
	select {
	case sub.Ch <- msg:
	default:
		log.Warn("collab: subscriber channel full, dropping message",
			"topic", msg.Topic, "agent", sub.AgentID)
	}
}

// replayHistory 回放匹配主题的历史消息
func (b *MessageBus) replayHistory(sub *Subscription) {
	defer func() {
		if r := recover(); r != nil {
			log.Debug("collab: replayHistory recovered from panic", "agent", sub.AgentID)
		}
	}()

	b.mu.RLock()
	defer b.mu.RUnlock()

	topicSet := make(map[string]bool, len(sub.Topics))
	for _, t := range sub.Topics {
		topicSet[t] = true
	}

	for _, msg := range b.history {
		if topicSet[msg.Topic] {
			select {
			case sub.Ch <- msg:
			default:
				return // 缓冲区满，停止回放
			}
		}
	}
}

// removeFromIndex 从主题索引中移除
func (b *MessageBus) removeFromIndex(agentID string, topics []string) {
	for _, topic := range topics {
		agents := b.topicIndex[topic]
		for i, id := range agents {
			if id == agentID {
				b.topicIndex[topic] = append(agents[:i], agents[i+1:]...)
				break
			}
		}
	}
}

// genMsgID 生成消息 ID
func (b *MessageBus) genMsgID() string {
	b.nextMsgID++
	return fmt.Sprintf("msg_%d", b.nextMsgID)
}

// SubscriberCount 返回当前订阅者数量
func (b *MessageBus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscriptions)
}

// TopicSubscriberCount 返回指定主题的订阅者数量
func (b *MessageBus) TopicSubscriberCount(topic string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.topicIndex[topic])
}

// Close 关闭消息总线
func (b *MessageBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, sub := range b.subscriptions {
		close(sub.Ch)
		delete(b.subscriptions, id)
	}
	b.topicIndex = make(map[string][]string)
	b.history = nil
}
