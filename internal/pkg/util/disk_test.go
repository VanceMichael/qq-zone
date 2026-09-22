// 目标卷容量查询测试

package util

import (
	"path/filepath"
	"testing"
)

func TestVolumeFreeSpace(t *testing.T) {
	dir := t.TempDir()

	mount, free, total, err := VolumeFreeSpace(dir)
	if err != nil {
		t.Fatalf("查询现存目录容量失败: %v", err)
	}
	if mount == "" || free <= 0 || total <= 0 {
		t.Fatalf("容量字段不合理: mount=%q free=%d total=%d", mount, free, total)
	}
	if free > total {
		t.Fatalf("可用空间 %d 不应大于总容量 %d", free, total)
	}

	// 尚不存在的深层路径应回退到现存祖先，而不是报错或创建目录。
	deep := filepath.Join(dir, "not", "created", "yet", "album")
	mount2, free2, _, err := VolumeFreeSpace(deep)
	if err != nil {
		t.Fatalf("不存在路径回退失败: %v", err)
	}
	if mount2 != mount || free2 != free {
		t.Fatalf("回退祖先 (%q,%d) 与现存目录 (%q,%d) 结果不一致", mount2, free2, mount, free)
	}
}
