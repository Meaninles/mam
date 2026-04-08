//go:build !windows

package storage

import "syscall"

func detectDiskUsage(path string) (total int64, available int64, err error) {
	target := resolveDiskUsageTarget(path)

	var stat syscall.Statfs_t
	if err := syscall.Statfs(target, &stat); err != nil {
		return 0, 0, err
	}

	blockSize := int64(stat.Bsize)
	return int64(stat.Blocks) * blockSize, int64(stat.Bavail) * blockSize, nil
}
