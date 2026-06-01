//go:build windows

package local

import (
	"fmt"

	"github.com/yxd/yunxi-home/internal/nas/base"
)

// GetDiskInfo 获取磁盘使用信息 (Windows)
func (s *FileSystem) GetDiskInfo(requestPath string) (*base.DiskInfo, error) {
	absPath, err := s.resolve(requestPath)
	if err != nil {
		return nil, err
	}

	// Windows 暂不支持 syscall.Statfs_t，返回基本信息
	return &base.DiskInfo{
		Path:    absPath,
		Total:   0,
		Free:    0,
		Used:    0,
		UsedPct: 0,
	}, fmt.Errorf("Windows 磁盘信息暂不支持")
}
