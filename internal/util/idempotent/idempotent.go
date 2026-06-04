// Package idempotent provides guards to ensure operations can be safely retried
// or reversed without causing duplicate side effects.
package idempotent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Key generates a deterministic idempotency key from operation name and arguments.
// The same (operation, args) always produces the same key.
func Key(operation string, args interface{}) string {
	h := sha256.New()
	h.Write([]byte(operation))
	if args != nil {
		b, _ := json.Marshal(args)
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ── In-flight registry ─────────────────────────────────────────────────

// Registry tracks in-flight operations to prevent duplicate execution.
// Operations with the same idempotency key will be deduplicated.
type Registry struct {
	mu      sync.Mutex
	entries map[string]*entry
}

type entry struct {
	result   interface{}
	err      error
	done     chan struct{}
	expireAt time.Time
}

// NewRegistry creates a new in-flight registry.
func NewRegistry() *Registry {
	r := &Registry{entries: make(map[string]*entry)}
	go r.cleanupLoop()
	return r
}

// Do ensures the operation runs exactly once for the given key.
// If another caller is already executing the same operation, this call
// waits for the existing result instead of executing again.
func (r *Registry) Do(key string, ttl time.Duration, fn func() (interface{}, error)) (interface{}, error) {
	r.mu.Lock()
	if existing, ok := r.entries[key]; ok {
		if time.Now().Before(existing.expireAt) {
			ch := existing.done
			r.mu.Unlock()
			<-ch // Wait for the in-flight operation
			// Re-read result under lock
			r.mu.Lock()
			e := r.entries[key]
			result, err := e.result, e.err
			r.mu.Unlock()
			return result, err
		}
	}
	// Create new entry
	e := &entry{
		done:     make(chan struct{}),
		expireAt: time.Now().Add(ttl),
	}
	r.entries[key] = e
	r.mu.Unlock()

	// Execute
	result, err := fn()

	// Store result and signal waiters
	r.mu.Lock()
	e.result = result
	e.err = err
	close(e.done)
	r.mu.Unlock()

	return result, err
}

// Cleanup removes an entry after it's no longer needed.
func (r *Registry) Cleanup(key string) {
	r.mu.Lock()
	delete(r.entries, key)
	r.mu.Unlock()
}

// cleanupLoop periodically removes expired entries.
func (r *Registry) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		for key, e := range r.entries {
			if now.After(e.expireAt) {
				// Only remove if done channel is closed (operation completed)
				select {
				case <-e.done:
					delete(r.entries, key)
				default:
					// Still in-flight, keep it
				}
			}
		}
		r.mu.Unlock()
	}
}

// ── Result cache ───────────────────────────────────────────────────────

// Cache caches operation results for a TTL to enable safe retry without
// re-executing expensive or side-effect-heavy operations.
type Cache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

type cacheEntry struct {
	result   interface{}
	err      error
	expireAt time.Time
}

// NewCache creates a result cache.
func NewCache() *Cache {
	return &Cache{items: make(map[string]cacheEntry)}
}

// Get returns a cached result if available and not expired.
func (c *Cache) Get(key string) (interface{}, error, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[key]
	if !ok || time.Now().After(e.expireAt) {
		return nil, nil, false
	}
	return e.result, e.err, true
}

// Set stores a result with TTL.
func (c *Cache) Set(key string, result interface{}, err error, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Simple eviction: if cache is too large, clear stale entries
	if len(c.items) > 10000 {
		now := time.Now()
		for k, v := range c.items {
			if now.After(v.expireAt) {
				delete(c.items, k)
			}
		}
	}
	c.items[key] = cacheEntry{result: result, err: err, expireAt: time.Now().Add(ttl)}
}

// ── Reverse operation helper ───────────────────────────────────────────

// UndoStack records operations as (do, undo) pairs so they can be reversed.
type UndoStack struct {
	mu    sync.Mutex
	stack []UndoAction
}

// UndoAction represents a reversible operation.
type UndoAction struct {
	Description string
	Undo        func() error
}

// NewUndoStack creates an undo stack.
func NewUndoStack() *UndoStack {
	return &UndoStack{}
}

// Push records an action that can be undone.
func (us *UndoStack) Push(desc string, undo func() error) {
	us.mu.Lock()
	defer us.mu.Unlock()
	us.stack = append(us.stack, UndoAction{Description: desc, Undo: undo})
}

// UndoAll reverses all recorded actions in LIFO order.
// Returns the first error encountered, but continues undoing remaining actions.
func (us *UndoStack) UndoAll(ctx context.Context) error {
	us.mu.Lock()
	actions := make([]UndoAction, len(us.stack))
	copy(actions, us.stack)
	us.stack = us.stack[:0] // Clear the stack
	us.mu.Unlock()

	var firstErr error
	// Undo in reverse order
	for i := len(actions) - 1; i >= 0; i-- {
		select {
		case <-ctx.Done():
			return fmt.Errorf("undo cancelled: %w", ctx.Err())
		default:
		}
		if err := actions[i].Undo(); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("撤销 '%s' 失败: %w", actions[i].Description, err)
			}
		}
	}
	return firstErr
}

// Clear discards all recorded actions (used after successful commit).
func (us *UndoStack) Clear() {
	us.mu.Lock()
	defer us.mu.Unlock()
	us.stack = nil
}

// Len returns the number of pending undo actions.
func (us *UndoStack) Len() int {
	us.mu.Lock()
	defer us.mu.Unlock()
	return len(us.stack)
}
