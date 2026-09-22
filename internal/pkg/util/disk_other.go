//go:build !windows

// 非 Windows 平台通过 statfs 读取卷可用容量。

package util

import "golang.org/x/sys/unix"

func diskFree(path string) (free, total int64, err error) {
	var st unix.Statfs_t
	if err = unix.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	blockSize := int64(st.Bsize)
	// Bavail 是普通用户（非 root）实际可用块数，比 Bfree 更保守，容量门槛用它才不会误判。
	return int64(st.Bavail) * blockSize, int64(st.Blocks) * blockSize, nil
}
