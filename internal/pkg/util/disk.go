// 目标卷可用空间查询。备份预检在创建任何目录之前就要拿到容量，
// 因此目标路径不存在时要向上找到最近的现存目录，再对所在卷做 Statfs。

package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// VolumeFreeSpace 返回 path 所在卷的挂载点（实际用于查询的现存目录）、
// 普通用户可用字节数和卷总字节数。整个过程只读，不创建任何目录或文件。
// path 的所有祖先都不存在（例如整份 storage 目录尚未生成）时会回退到当前工作目录。
func VolumeFreeSpace(path string) (mount string, free, total int64, err error) {
	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		return "", 0, 0, absErr
	}

	dir := abs
	for {
		fi, statErr := os.Stat(dir)
		if statErr == nil && fi.IsDir() {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// 理论上不会发生（退到根目录一定存在），兜底用当前工作目录所在卷。
			dir, _ = os.Getwd()
			break
		}
		dir = parent
	}

	free, total, err = diskFree(dir)
	if err != nil {
		return "", 0, 0, fmt.Errorf("查询目录 %s 所在卷容量失败: %w", dir, err)
	}
	return dir, free, total, nil
}
