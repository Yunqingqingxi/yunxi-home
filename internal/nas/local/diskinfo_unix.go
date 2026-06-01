//go:build !windows

package local

import (
	"fmt"
	"syscall"

	"github.com/yxd/yunxi-home/internal/nas/base"
)

// GetDiskInfo 获取磁盘使用信息 (Unix)
func (s *FileSystem) GetDiskInfo(requestPath string) (*base.DiskInfo, error) {
	absPath, err := s.resolve(requestPath)
	if err != nil {
		return nil, err
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(absPath, &stat); err != nil {
		return nil, fmt.Errorf("获取磁盘信息失败: %w", err)
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	used := total - free
	usedPct := float64(0)
	if total > 0 {
		usedPct = float64(used) / float64(total) * 100
	}

	return &base.DiskInfo{
		Path:    absPath,
		Total:   total,
		Free:    free,
		Used:    used,
		UsedPct: usedPct,
	}, nil
}
