//go:build !windows

package local

import (
	"io/fs"
	"strconv"
	"syscall"

	"github.com/yxd/yunxi-home/internal/nas/base"
)

func fillOwnerGroupFromSys(info fs.FileInfo, st *base.FileStat) {
	if statT, ok := info.Sys().(*syscall.Stat_t); ok {
		st.Owner = strconv.FormatUint(uint64(statT.Uid), 10)
		st.Group = strconv.FormatUint(uint64(statT.Gid), 10)
	}
}
