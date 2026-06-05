// Package session provides continuous-session message queue for Claude Code-style
// persistent conversations. Messages are queued and processed sequentially by the
// main agent loop, which never truly "ends" until explicitly closed.
package session

import (
	"sync"
	"time"
)

// QueuedMessage represents a message waiting to be processed by the agent loop.
type QueuedMessage struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`       // "user" | "system" | "agent_result"
	Content   string    `json:"content"`
	Priority  int       `json:"priority"`   // 0=normal, 1=high (user interrupt), 2=system
	Source    string    `json:"source"`     // "user" | "inject" | "agent" | "system"
	CreatedAt time.Time `json:"created_at"`
	Done      chan struct{} `json:"-"`      // closed when message is fully processed
}

// MessageQueue manages a priority queue of messages for a single session.
// Messages are processed in priority order (high first), then FIFO within same priority.
type MessageQueue struct {
	mu       sync.Mutex
	queue    []*QueuedMessage
	processing bool
	onProcess func(msg *QueuedMessage) // callback to process next message
}

// NewMessageQueue creates a message queue for a session.
func NewMessageQueue(onProcess func(msg *QueuedMessage)) *MessageQueue {
	return &MessageQueue{
		queue:    make([]*QueuedMessage, 0, 32),
		onProcess: onProcess,
	}
}

// Push adds a message to the queue and triggers processing if idle.
func (mq *MessageQueue) Push(msg *QueuedMessage) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	if msg.Done == nil {
		msg.Done = make(chan struct{})
	}

	// Insert in priority order
	inserted := false
	for i, existing := range mq.queue {
		if msg.Priority > existing.Priority {
			mq.queue = append(mq.queue[:i], append([]*QueuedMessage{msg}, mq.queue[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		mq.queue = append(mq.queue, msg)
	}

	// If idle, start processing
	if !mq.processing {
		mq.processing = true
		go mq.processLoop()
	}
}

// PushAndWait adds a message and blocks until it's processed.
func (mq *MessageQueue) PushAndWait(msg *QueuedMessage) {
	msg.Done = make(chan struct{})
	mq.Push(msg)
	<-msg.Done
}

// Len returns the number of queued messages.
func (mq *MessageQueue) Len() int {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	return len(mq.queue)
}

// IsProcessing returns true if a message is currently being processed.
func (mq *MessageQueue) IsProcessing() bool {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	return mq.processing
}

// Drain removes all queued messages.
func (mq *MessageQueue) Drain() {
	mq.mu.Lock()
	defer mq.mu.Unlock()
	// Signal all waiting messages as cancelled
	for _, msg := range mq.queue {
		if msg.Done != nil {
			close(msg.Done)
		}
	}
	mq.queue = mq.queue[:0]
	mq.processing = false
}

func (mq *MessageQueue) processLoop() {
	for {
		mq.mu.Lock()
		if len(mq.queue) == 0 {
			mq.processing = false
			mq.mu.Unlock()
			return
		}
		msg := mq.queue[0]
		mq.queue = mq.queue[1:]
		mq.mu.Unlock()

		if mq.onProcess != nil {
			mq.onProcess(msg)
		}

		if msg.Done != nil {
			close(msg.Done)
		}
	}
}

// ── Global queue registry ──────────────────────────────────────────────

// QueueRegistry manages per-session message queues.
type QueueRegistry struct {
	mu     sync.RWMutex
	queues map[string]*MessageQueue
}

// NewQueueRegistry creates a global queue registry.
func NewQueueRegistry() *QueueRegistry {
	return &QueueRegistry{queues: make(map[string]*MessageQueue)}
}

// GetOrCreate returns the queue for a session, creating one if needed.
func (qr *QueueRegistry) GetOrCreate(sessionID string, onProcess func(msg *QueuedMessage)) *MessageQueue {
	qr.mu.Lock()
	defer qr.mu.Unlock()
	if mq, ok := qr.queues[sessionID]; ok {
		return mq
	}
	mq := NewMessageQueue(onProcess)
	qr.queues[sessionID] = mq
	return mq
}

// Get returns the queue for a session, or nil.
func (qr *QueueRegistry) Get(sessionID string) *MessageQueue {
	qr.mu.RLock()
	defer qr.mu.RUnlock()
	return qr.queues[sessionID]
}

// Delete removes a session's queue.
func (qr *QueueRegistry) Delete(sessionID string) {
	qr.mu.Lock()
	defer qr.mu.Unlock()
	if mq, ok := qr.queues[sessionID]; ok {
		mq.Drain()
		delete(qr.queues, sessionID)
	}
}
