//go:build windows

// Windows 平台通过 GetDiskFreeSpaceEx 读取卷可用容量。

package util

import "golang.org/x/sys/windows"

func diskFree(path string) (free, total int64, err error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var (
		freeAvailable uint64
		totalBytes    uint64
		totalFree     uint64
	)
	if err = windows.GetDiskFreeSpaceEx(pathPtr, &freeAvailable, &totalBytes, &totalFree); err != nil {
		return 0, 0, err
	}
	return int64(freeAvailable), int64(totalBytes), nil
}
