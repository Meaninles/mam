//go:build windows

package storage

import "golang.org/x/sys/windows"

func detectDiskUsage(path string) (total int64, available int64, err error) {
	target := resolveDiskUsageTarget(path)

	pointer, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return 0, 0, err
	}

	var (
		freeBytesAvailable uint64
		totalBytes         uint64
		totalFreeBytes     uint64
	)
	if err := windows.GetDiskFreeSpaceEx(pointer, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err != nil {
		return 0, 0, err
	}

	return int64(totalBytes), int64(totalFreeBytes), nil
}
