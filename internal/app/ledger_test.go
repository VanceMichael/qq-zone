// 完整性账本的模型、原子存储、身份构造与域根发现测试（全程本地，无网络）。

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qinjintian/qq-zone/internal/pkg/util"
)

// writeMedia 在 root/rel 写入内容，返回其 size 与 sha256。
func writeMedia(t *testing.T, root, rel, content string) (int64, string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), os.ModePerm); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write media: %v", err)
	}
	size, sum, err := util.FileDigest(p)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	return size, sum
}

func TestLedgerSourceIDFormats(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{AlbumSourceID("10001", "albumid-9", "sloc-xyz"), "album:10001:albumid-9:sloc-xyz"},
		{GroupSourceID("10001", "555群", "aid", "sl"), "group:10001:555群:aid:sl"},
		{ShuoshuoSourceID("10001", "tid1", "pic2", ""), "shuoshuo:10001:tid1:pic2"},
		{BoardSourceID("10001", "tid1", "pic2", ""), "board:10001:tid1:pic2"},
		{ShuoshuoSourceID("10001", "tid1", "", "http://x/y"), "shuoshuo:10001:tid1:url:" + util.MD5("http://x/y")},
		{LocalSourceID("a/b/c.jpg"), "local:a/b/c.jpg"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Fatalf("source id = %q, want %q", c.got, c.want)
		}
	}
	if !IsLocalSource("local:a/b.jpg") || IsLocalSource("album:1:2:3") {
		t.Fatal("IsLocalSource 判断错误")
	}
}

func TestMediaKindFromExt(t *testing.T) {
	cases := map[string]string{
		"a.jpg": MediaKindImage, "A.PNG": MediaKindImage, "v.mp4": MediaKindVideo,
		"a.mp3": MediaKindVoice, "x.bin": MediaKindOther,
	}
	for name, want := range cases {
		if got := MediaKindFromExt(name); got != want {
			t.Fatalf("%s -> %s, want %s", name, got, want)
		}
	}
}

func TestLedgerUpsertAndReplaceBaseline(t *testing.T) {
	root := t.TempDir()
	l := NewLedger(LedgerKindAlbum, "10001", "", root)

	size, sum := writeMedia(t, root, "相册/2021/03/IMG_x.jpg", "hello")
	base := LedgerEntry{
		SourceID: LocalSourceID("相册/2021/03/IMG_x.jpg"), MediaKind: MediaKindImage,
		RelPath: "相册/2021/03/IMG_x.jpg", Size: size, SHA256: sum, OriginVerified: false,
	}
	l.Upsert(base)
	if l.Count() != 1 {
		t.Fatalf("baseline entries = %d", l.Count())
	}

	// 同一 source 重复 upsert：去重且保留 CreatedAt。
	firstCreated := l.Entries[0].CreatedAt
	time.Sleep(2 * time.Millisecond)
	l.Upsert(base)
	if l.Count() != 1 {
		t.Fatalf("upsert 应按 source 去重，实际 %d", l.Count())
	}
	if !l.Entries[0].CreatedAt.Equal(firstCreated) {
		t.Fatal("重复 upsert 不应覆盖 CreatedAt")
	}

	// 同 rel_path 的已验证在线条目接管：local 条目被替换。
	verified := LedgerEntry{
		SourceID: AlbumSourceID("10001", "aid", "sloc1"), MediaKind: MediaKindImage,
		RelPath: "相册/2021/03/IMG_x.jpg", Size: size, SHA256: sum, OriginVerified: true,
	}
	l.Upsert(verified)
	if l.Count() != 1 {
		t.Fatalf("在线条目应替换同路径 local 条目，实际 %d", l.Count())
	}
	got, ok := l.FindByRelPath("相册/2021/03/IMG_x.jpg")
	if !ok || !got.OriginVerified || got.SourceID != verified.SourceID {
		t.Fatalf("同路径替换错误: %+v ok=%v", got, ok)
	}
}

// 同一来源条目改 rel（扩展名变更）后，旧 rel 键必须立即失效（R2 advisory A1）。
func TestLedgerUpsertSourceRenameDropsOldRelIndex(t *testing.T) {
	root := t.TempDir()
	l := NewLedger(LedgerKindAlbum, "10001", "", root)
	id := AlbumSourceID("10001", "a1", "sloc1")
	l.Upsert(LedgerEntry{SourceID: id, RelPath: "A/a.jpg", SHA256: "x"})
	l.Upsert(LedgerEntry{SourceID: id, RelPath: "A/a.png", SHA256: "y", OriginVerified: true})

	if _, ok := l.FindByRelPath("A/a.jpg"); ok {
		t.Fatal("旧 rel 键不应残留")
	}
	e, ok := l.FindByRelPath("A/a.png")
	if !ok || e.SourceID != id {
		t.Fatalf("新 rel 键应指向同一条目: %+v ok=%v", e, ok)
	}
	if l.Count() != 1 {
		t.Fatalf("同来源仍只应有 1 条，实际 %d", l.Count())
	}
	if got := l.SkipDecision(id); got != LedgerSkipMismatch {
		t.Fatal("改名后磁盘无新文件，决策应为 mismatch（实际文件未放置）")
	}
}

func TestLedgerRemoveByRelPathPrefix(t *testing.T) {
	root := t.TempDir()
	l := NewLedger(LedgerKindAlbum, "10001", "", root)
	l.Upsert(LedgerEntry{SourceID: "album:1:a:1", RelPath: "A/x.jpg", SHA256: "1"})
	l.Upsert(LedgerEntry{SourceID: "album:1:a:2", RelPath: "A/y.jpg", SHA256: "2"})
	l.Upsert(LedgerEntry{SourceID: "album:1:b:1", RelPath: "B/z.jpg", SHA256: "3"})

	if n := l.RemoveByRelPathPrefix("A/"); n != 2 {
		t.Fatalf("删除 A/ 前缀条目数 = %d, want 2", n)
	}
	if l.Count() != 1 {
		t.Fatalf("剩余条目 = %d, want 1", l.Count())
	}
	if _, ok := l.FindBySource("album:1:b:1"); !ok {
		t.Fatal("B 相册条目应保留")
	}
}

func TestLedgerAtomicSaveLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	l := NewLedger(LedgerKindShuoshuo, "10001", "", root)
	size, sum := writeMedia(t, root, "media/2021/03/a.jpg", "abc")
	l.Upsert(LedgerEntry{
		SourceID: ShuoshuoSourceID("10001", "tid", "mid", ""),
		RelPath:  "media/2021/03/a.jpg", Size: size, SHA256: sum,
		OriginVerified: true, MediaKind: MediaKindImage,
	})
	if err := l.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(root, integrityDirName))
	if err != nil {
		t.Fatalf("read integrity dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("原子写完成后不应残留临时文件: %s", e.Name())
		}
	}

	got, err := LoadLedger(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Kind != LedgerKindShuoshuo || got.HashAlg != ledgerHashAlg || got.Count() != 1 {
		t.Fatalf("重新加载内容不符: %+v", got)
	}
	if got.Entries[0].SHA256 != sum || got.Entries[0].Size != size {
		t.Fatal("条目 size/sha256 往返不一致")
	}
}

func TestLoadLedgerMissing(t *testing.T) {
	if _, err := LoadLedger(t.TempDir()); err != ErrLedgerMissing {
		t.Fatalf("无账本应返回 ErrLedgerMissing，实际 %v", err)
	}
}

func TestLoadLedgerFallbackToBackup(t *testing.T) {
	root := t.TempDir()
	l := NewLedger(LedgerKindBoard, "10001", "", root)
	l.Upsert(LedgerEntry{SourceID: BoardSourceID("10001", "t", "m", ""), RelPath: "media/a.jpg", SHA256: "aa"})
	if err := l.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	// 写坏 ledger.json，bak 仍应可解析。
	if err := os.WriteFile(LedgerPath(root), []byte("{broken json"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLedger(root)
	if err != nil {
		t.Fatalf("损坏 ledger.json 应回退 bak: %v", err)
	}
	if got.Count() != 1 || got.Entries[0].SHA256 != "aa" {
		t.Fatalf("回退内容错误: %+v", got)
	}
}

func TestLoadLedgerBothCorrupt(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, integrityDirName)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ledgerFileName), []byte("{x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ledgerBackupFileName), []byte("}y"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLedger(root); !errors.Is(err, ErrLedgerCorrupt) {
		t.Fatalf("双损坏应返回 ErrLedgerCorrupt，实际 %v", err)
	}
}

func TestDiscoverLedgerRoots(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	// 四类域根各建一个。
	dirs := []string{
		AlbumLedgerRoot("10001"),
		GroupLedgerRoot("10001", "90001"),
		ShuoshuoLedgerRoot("10001"),
		BoardLedgerRoot("10002"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, os.ModePerm); err != nil {
			t.Fatal(err)
		}
	}
	// 给群相册写一本账，验证状态描述。
	gl := NewLedger(LedgerKindGroupAlbum, "10001", "90001", GroupLedgerRoot("10001", "90001"))
	gl.Upsert(LedgerEntry{SourceID: "group:10001:90001:a:s", RelPath: "相册A/x.jpg", SHA256: "z"})
	if err := gl.Save(); err != nil {
		t.Fatal(err)
	}
	// 损坏账本：无 bak 且 json 坏掉 → Corrupt。
	badRoot := AlbumLedgerRoot("10003")
	_ = os.MkdirAll(filepath.Join(badRoot, integrityDirName), os.ModePerm)
	_ = os.WriteFile(LedgerPath(badRoot), []byte("{"), 0644)

	domains := discoverLedgerRoots("storage")
	seen := map[LedgerKind]int{}
	var groupDomain, badDomain *LedgerDomain
	for i := range domains {
		d := &domains[i]
		seen[d.Kind]++
		if d.Kind == LedgerKindGroupAlbum {
			groupDomain = d
		}
		if d.OwnerUin == "10003" {
			badDomain = d
		}
	}
	if seen[LedgerKindAlbum] != 2 || seen[LedgerKindGroupAlbum] != 1 ||
		seen[LedgerKindShuoshuo] != 1 || seen[LedgerKindBoard] != 1 {
		t.Fatalf("域根发现数量错误: %+v", seen)
	}
	if groupDomain == nil || !groupDomain.HasLedger || groupDomain.EntryCount != 1 || groupDomain.GroupID != "90001" {
		t.Fatalf("群域根状态错误: %+v", groupDomain)
	}
	if badDomain == nil || !badDomain.HasLedger || !badDomain.Corrupt {
		t.Fatalf("损坏域根状态错误: %+v", badDomain)
	}
}

// TestLedgerSaveKeepsIndexesValidAcrossAlbums 回归审查 F1：
// 跨相册乱序条目 Save（原地排序）后，内存下标必须仍指向各自条目，
// 后续相册的 SkipDecision/Upsert 不得串号或误删他相册健康条目。
func TestLedgerSaveKeepsIndexesValidAcrossAlbums(t *testing.T) {
	root := t.TempDir()
	a0 := AlbumSourceID("10001", "A", "a0")
	a1 := AlbumSourceID("10001", "A", "a1")
	b0 := AlbumSourceID("10001", "B", "b0")
	b1 := AlbumSourceID("10001", "B", "b1")

	// 故意乱序插入（实际下载完成顺序 ≠ 落盘排序顺序）。
	l := NewLedger(LedgerKindAlbum, "10001", "", root)
	l.Upsert(LedgerEntry{SourceID: b1, RelPath: "B/b1.jpg", SHA256: "b1"})
	l.Upsert(LedgerEntry{SourceID: a0, RelPath: "A/a0.jpg", SHA256: "a0"})
	l.Upsert(LedgerEntry{SourceID: b0, RelPath: "B/b0.jpg", SHA256: "b0"})
	l.Upsert(LedgerEntry{SourceID: a1, RelPath: "A/a1.jpg", SHA256: "a1"})
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}

	// 排序后 FindBySource/RelPath 必须各归各位。
	for _, want := range []struct{ sid, rel string }{
		{a0, "A/a0.jpg"}, {a1, "A/a1.jpg"}, {b0, "B/b0.jpg"}, {b1, "B/b1.jpg"},
	} {
		e, ok := l.FindBySource(want.sid)
		if !ok || e.RelPath != want.rel {
			t.Fatalf("Save 后下标串号: source %s -> %+v ok=%v", want.sid, e, ok)
		}
		if e2, ok := l.FindByRelPath(want.rel); !ok || e2.SourceID != want.sid {
			t.Fatalf("Save 后 rel 下标串号: %s -> %+v ok=%v", want.rel, e2, ok)
		}
	}

	// 模拟二次备份：B0 文件被删（缺失），A 相册新完成 A2 并再次提交。
	l.Upsert(LedgerEntry{SourceID: AlbumSourceID("10001", "A", "a2"), RelPath: "A/a2.jpg", SHA256: "a2", OriginVerified: true})
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	if got := l.SkipDecision(b0); got != LedgerSkipMismatch {
		t.Fatalf("B0 文件缺失必须 mismatch（审查 F1 场景），实际 %d", got)
	}
	// B 的两条健康条目仍在账本中。
	if _, ok := l.FindBySource(b0); !ok {
		t.Fatal("B0 条目不得被 A2 的 Upsert 误删")
	}
	if _, ok := l.FindBySource(b1); !ok {
		t.Fatal("B1 条目不得被 A2 的 Upsert 误删")
	}
	if l.Count() != 5 {
		t.Fatalf("条目数 = %d, want 5", l.Count())
	}
}

func TestEntryFromFile(t *testing.T) {
	root := t.TempDir()
	content := "integrity-content"
	size, sum := writeMedia(t, root, "album名/2022/01/v.mp4", content)

	e, err := EntryFromFile(root, "album名/2022/01/v.mp4", AlbumSourceID("1", "2", "3"), true)
	if err != nil {
		t.Fatal(err)
	}
	if e.Size != size || e.SHA256 != sum || !e.OriginVerified || e.MediaKind != MediaKindVideo {
		t.Fatalf("条目构造错误: %+v", e)
	}
	want := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	if e.SHA256 != want || hex.EncodedLen(32) != len(want) {
		t.Fatalf("sha256 = %s, want %s", e.SHA256, want)
	}
}
