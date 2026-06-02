package resilience

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// ── 哨兵错误（替代字符串匹配）──

var (
	ErrTimeout      = errors.New("timeout")
	ErrNetwork      = errors.New("network error")
	ErrRateLimit    = errors.New("rate limit exceeded")
	ErrTemporary    = errors.New("temporary unavailable")
	ErrInvalidArgs  = errors.New("invalid arguments")
	ErrPermission   = errors.New("permission denied")
	ErrNotFound     = errors.New("not found")
	ErrQuotaExceed  = errors.New("quota exceeded")
	ErrExecFailed   = errors.New("execution failed")
)

// IsRetryable 判断错误是否可重试
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrTimeout):
		return true
	case errors.Is(err, ErrNetwork):
		return true
	case errors.Is(err, ErrRateLimit):
		return true
	case errors.Is(err, ErrTemporary):
		return true
	default:
		return false
	}
}

// IsFatal 判断错误是否致命（不可恢复）
func IsFatal(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, ErrInvalidArgs):
		return true
	case errors.Is(err, ErrPermission):
		return true
	case errors.Is(err, ErrQuotaExceed):
		return true
	default:
		return false
	}
}

// RetryPolicy 重试策略配置
type RetryPolicy struct {
	MaxRetries int           // 最大重试次数
	Backoff    time.Duration // 基础退避时间
	MaxBackoff time.Duration // 最大退避时间
	Jitter     float64       // 抖动因子 [0, 1]
}

// DefaultRetryPolicy 默认重试策略
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 2,
		Backoff:    1 * time.Second,
		MaxBackoff: 30 * time.Second,
		Jitter:     0.1,
	}
}

// NoRetry 不重试策略
func NoRetry() RetryPolicy {
	return RetryPolicy{MaxRetries: 0}
}

// Do 执行带重试的操作
func Do(ctx context.Context, policy RetryPolicy, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// 不可重试的错误，立即返回
		if IsFatal(err) {
			return err
		}
		if !IsRetryable(err) {
			return err
		}

		// 最后一次尝试不需要等待
		if attempt < policy.MaxRetries {
			backoff := calculateBackoff(policy, attempt)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
			}
		}
	}

	return fmt.Errorf("all %d retries exhausted: %w", policy.MaxRetries+1, lastErr)
}

// DoWithResult 执行带重试的操作（带返回值）
func DoWithResult[T any](ctx context.Context, policy RetryPolicy, fn func() (T, error)) (T, error) {
	var lastErr error
	var zero T

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		if IsFatal(err) || !IsRetryable(err) {
			return zero, err
		}

		if attempt < policy.MaxRetries {
			backoff := calculateBackoff(policy, attempt)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return zero, fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
			}
		}
	}

	return zero, fmt.Errorf("all %d retries exhausted: %w", policy.MaxRetries+1, lastErr)
}

// calculateBackoff 计算指数退避 + 随机抖动
func calculateBackoff(policy RetryPolicy, attempt int) time.Duration {
	// 指数退避: backoff * 2^attempt
	backoff := float64(policy.Backoff) * math.Pow(2, float64(attempt))

	// 限制最大退避
	if policy.MaxBackoff > 0 && backoff > float64(policy.MaxBackoff) {
		backoff = float64(policy.MaxBackoff)
	}

	// 添加抖动: backoff * (1 ± jitter)
	if policy.Jitter > 0 {
		jitter := (rand.Float64()*2 - 1) * policy.Jitter // [-jitter, +jitter]
		backoff = backoff * (1 + jitter)
	}

	if backoff < 0 {
		backoff = 0
	}

	return time.Duration(backoff)
}

// ── 工具类型重试配置建议 ──

// RetryConfigForTool 根据工具类型返回推荐的重试策略
func RetryConfigForTool(riskLevel, category string) RetryPolicy {
	switch {
	case riskLevel == "readonly":
		return RetryPolicy{MaxRetries: 2, Backoff: 1 * time.Second, MaxBackoff: 10 * time.Second, Jitter: 0.1}
	case riskLevel == "mutation":
		return RetryPolicy{MaxRetries: 1, Backoff: 2 * time.Second, MaxBackoff: 10 * time.Second, Jitter: 0.1}
	case riskLevel == "dangerous":
		return NoRetry() // 危险操作不重试
	case category == "mcp":
		return RetryPolicy{MaxRetries: 1, Backoff: 2 * time.Second, MaxBackoff: 15 * time.Second, Jitter: 0.1}
	default:
		return DefaultRetryPolicy()
	}
}
