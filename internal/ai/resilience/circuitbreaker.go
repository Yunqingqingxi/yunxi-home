// Package resilience 提供熔断器、重试和错误分类等弹性策略组件。
package resilience

import (
	"errors"
	"github.com/Yunqingqingxi/yunxi-home/internal/logger"
	"sync"
	"time"
)

// ErrCircuitOpen 熔断器打开时返回的错误
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CBState 熔断器状态
type CBState string

const (
	CBClosed   CBState = "closed"    // 正常：允许请求通过
	CBOpen     CBState = "open"      // 熔断：拒绝所有请求
	CBHalfOpen CBState = "half_open" // 半开：允许少量试探请求
)

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu           sync.Mutex
	state        CBState
	failures     int
	successes    int
	lastFailure  time.Time
	threshold    int           // 连续失败阈值（默认 5）
	cooldown     time.Duration // 冷却时间（默认 30s）
	halfOpenMax  int           // HalfOpen 最多允许的试探请求数（默认 1）
	name         string        // 用于日志标识
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Name        string
	Threshold   int
	Cooldown    time.Duration
	HalfOpenMax int
}

// DefaultCircuitBreakerConfig 默认配置
func DefaultCircuitBreakerConfig(name string) CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Name:        name,
		Threshold:   5,
		Cooldown:    30 * time.Second,
		HalfOpenMax: 1,
	}
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 5
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = 30 * time.Second
	}
	if cfg.HalfOpenMax <= 0 {
		cfg.HalfOpenMax = 1
	}
	return &CircuitBreaker{
		state:       CBClosed,
		threshold:   cfg.Threshold,
		cooldown:    cfg.Cooldown,
		halfOpenMax: cfg.HalfOpenMax,
		name:        cfg.Name,
	}
}

// State 返回当前状态
func (cb *CircuitBreaker) State() CBState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.tryTransition()
	return cb.state
}

// Call 执行操作（带熔断保护）
func (cb *CircuitBreaker) Call(fn func() error) error {
	// 检查是否可以执行
	if !cb.allowRequest() {
		return ErrCircuitOpen
	}

	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

// allowRequest 检查是否允许请求通过
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CBClosed:
		return true

	case CBOpen:
		// 检查是否可以进入 HalfOpen
		if time.Since(cb.lastFailure) >= cb.cooldown {
			cb.state = CBHalfOpen
			cb.successes = 0
			log.Info("circuit breaker: open → half_open", "name", cb.name)
			return true
		}
		return false

	case CBHalfOpen:
		// 只允许有限试探请求
		return cb.successes < cb.halfOpenMax
	}

	return false
}

// onSuccess 记录成功
func (cb *CircuitBreaker) onSuccess() {
	cb.successes++

	switch cb.state {
	case CBHalfOpen:
		// 试探成功 → 恢复
		if cb.successes >= cb.halfOpenMax {
			cb.state = CBClosed
			cb.failures = 0
			log.Info("circuit breaker: half_open → closed (recovered)", "name", cb.name)
		}
	case CBClosed:
		cb.failures = 0 // 成功后重置失败计数
	}
}

// onFailure 记录失败
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailure = time.Now()

	switch cb.state {
	case CBClosed:
		if cb.failures >= cb.threshold {
			cb.state = CBOpen
			log.Warn("circuit breaker: closed → open",
				"name", cb.name,
				"failures", cb.failures,
				"threshold", cb.threshold,
			)
		}
	case CBHalfOpen:
		// 试探失败 → 重新打开
		cb.state = CBOpen
		log.Warn("circuit breaker: half_open → open (trial failed)", "name", cb.name)
	}
}

// tryTransition 尝试状态迁移（基于时间）
func (cb *CircuitBreaker) tryTransition() {
	if cb.state == CBOpen && time.Since(cb.lastFailure) >= cb.cooldown {
		cb.state = CBHalfOpen
		cb.successes = 0
		log.Info("circuit breaker: open → half_open (cooldown)", "name", cb.name)
	}
}

// Reset 重置熔断器到 Closed 状态
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CBClosed
	cb.failures = 0
	cb.successes = 0
	log.Info("circuit breaker reset", "name", cb.name)
}

// Stats 返回统计信息
func (cb *CircuitBreaker) Stats() CBStats {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.tryTransition()
	return CBStats{
		Name:     cb.name,
		State:    cb.state,
		Failures: cb.failures,
		LastFail: cb.lastFailure,
	}
}

// CBStats 熔断器统计
type CBStats struct {
	Name     string    `json:"name"`
	State    CBState   `json:"state"`
	Failures int       `json:"failures"`
	LastFail time.Time `json:"last_fail,omitempty"`
}
