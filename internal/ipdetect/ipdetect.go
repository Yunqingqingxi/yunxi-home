// Package ipdetect 统一 IP 检测模块入口。
//
// 消费者只需导入此包即可使用 IP 检测功能，无需关心底层实现。
//
//	import "github.com/yxd/yunxi-home/internal/ipdetect"
//	detector := ipdetect.NewDetector(&ipdetect.DetectorConfig{...})
//	ip, err := detector.GetCurrentIPv6(ctx)
package ipdetect

import (
	"github.com/yxd/yunxi-home/internal/ipdetect/base"
	"github.com/yxd/yunxi-home/internal/ipdetect/multisource"
)

// ── 类型别名（消费者直接使用 ipdetect.Detector, ipdetect.DetectorConfig 等）─────────

// Detector IP 检测器接口
type Detector = base.Detector

// DetectorConfig IP 检测器配置
type DetectorConfig = multisource.Config

// Source IP 数据源配置
type Source = multisource.Source

// ── 工厂函数 ─────────────────────────────────────────────────

// NewDetector 创建多源 IP 检测器，返回 Detector 接口
func NewDetector(cfg *DetectorConfig) Detector {
	return multisource.New(cfg)
}
