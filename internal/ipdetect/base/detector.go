// Package base 定义 IP 检测模块的通用类型和接口，零外部依赖。
package base

import "context"

// Detector IP 检测器接口
type Detector interface {
	GetCurrentIPv6(ctx context.Context) (string, error)
	GetCurrentIPv4(ctx context.Context) (string, error)
	GetCachedIP(domain string) (string, bool)
	SetCachedIP(domain, ip string)
}
