//go:build !windows

package security

import (
	"os"
	"syscall"
	"time"
)

// fileAccessTime returns the access time of a file.
// Returns zero time if atime is unavailable.
func fileAccessTime(info os.FileInfo) time.Time {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}
	}

	return time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
}
