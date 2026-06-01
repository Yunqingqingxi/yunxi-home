//go:build windows

package local

import (
	"io/fs"

	"github.com/Yunqingqingxi/yunxi-home/internal/nas/base"
)

func fillOwnerGroupFromSys(_ fs.FileInfo, _ *base.FileStat) {
	// Windows does not have Unix-style owner/group via syscall.Stat_t.
	// Owner/Group fields remain empty on Windows.
}
