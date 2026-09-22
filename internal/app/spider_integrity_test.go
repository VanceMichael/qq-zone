// 相册 Spider 与完整性账本衔接的本地测试：跳过决策、暂存提交、全量替换、取消不提交。

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/qinjintian/qq-zone/internal/qzone"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func newTestSpiderLedger(t *testing.T, kind LedgerKind, groupID string) (*Spider, string) {
	t.Helper()
	root := t.TempDir()
	owner := "10001"
	l := NewLedger(kind, owner, groupID, root)
	s := &Spider{
		logger:     zap.NewNop().Sugar(),
		ledger:     l,
		ledgerRoot: root,
	}
	if kind == LedgerKindGroupAlbum {
		s.groupID = groupID
		s.client = &qzone.Client{QQ: owner}
	}
	return s, root
}

func TestLedgerSkipDecisionFourBranches(t *testing.T) {
	root := t.TempDir()
	l := NewLedger(LedgerKindAlbum, "10001", "", root)
	rel := "旅行/IMG_1.jpg"
	size, sum := writeMedia(t, root, rel, "photo-bytes")
	l.Upsert(LedgerEntry{
		SourceID: AlbumSourceID("10001", "album1", "sloc1"),
		RelPath:  rel, Size: size, SHA256: sum, OriginVerified: true, MediaKind: MediaKindImage,
	})

	// 1) 一致：离线健康，不触网。
	if got := l.SkipDecision(AlbumSourceID("10001", "album1", "sloc1")); got != LedgerSkipHealthy {
		t.Fatalf("一致文件应 healthy，实际 %d", got)
	}

	// 2) 字节被改：mismatch 必须重下。
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.WriteFile(p, []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := l.SkipDecision(AlbumSourceID("10001", "album1", "sloc1")); got != LedgerSkipMismatch {
		t.Fatalf("改动文件应 mismatch，实际 %d", got)
	}

	// 3) 文件缺失：mismatch。
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if got := l.SkipDecision(AlbumSourceID("10001", "album1", "sloc1")); got != LedgerSkipMismatch {
		t.Fatalf("缺失文件应 mismatch，实际 %d", got)
	}

	// 4) 无条目：交给旧逻辑（HEAD/非空），且不得声称账本健康。
	if got := l.SkipDecision(AlbumSourceID("10001", "album1", "other-sloc")); got != LedgerSkipNoEntry {
		t.Fatalf("无条目应 noEntry，实际 %d", got)
	}
}

func TestSpiderStageAndCommitAlbum(t *testing.T) {
	s, root := newTestSpiderLedger(t, LedgerKindAlbum, "")
	albumPath := filepath.Join(root, "旅行")

	// 账本里先放同相册的旧条目（模拟全量重下前的状态）和另一个相册的条目。
	s.ledger.Upsert(LedgerEntry{SourceID: "album:10001:old:gone", RelPath: "旅行/old.jpg", SHA256: "x"})
	s.ledger.Upsert(LedgerEntry{SourceID: "album:10001:keep:1", RelPath: "生活/keep.jpg", SHA256: "y"})
	// 同路径基线 local 条目，在线登记后应被替换。
	s.ledger.Upsert(LedgerEntry{SourceID: LocalSourceID("旅行/IMG_20200101_a.jpg"), RelPath: "旅行/IMG_20200101_a.jpg", SHA256: "base", OriginVerified: false})

	writeMedia(t, root, "旅行/IMG_20200101_a.jpg", "new-a")
	writeMedia(t, root, "旅行/2020/01/VID_b.mp4", "video-b")

	s.stageCompletedMedia(AlbumSourceID("10001", "album1", "slocA"), filepath.Join(root, "旅行/IMG_20200101_a.jpg"))
	s.stageCompletedMedia(AlbumSourceID("10001", "album1", "slocB"), filepath.Join(root, "旅行/2020/01/VID_b.mp4"))
	s.commitAlbumLedger(albumPath, true, true) // 全量、干净边界

	reloaded, err := LoadLedger(root)
	if err != nil {
		t.Fatalf("提交后重新加载: %v", err)
	}
	if reloaded.Count() != 3 {
		t.Fatalf("条目数 = %d（应为：旅行2 + 生活1）", reloaded.Count())
	}
	if _, ok := reloaded.FindBySource("album:10001:old:gone"); ok {
		t.Fatal("全量替换后旧前缀条目应被移除")
	}
	if _, ok := reloaded.FindBySource("album:10001:keep:1"); !ok {
		t.Fatal("其他相册条目必须保留")
	}
	e, ok := reloaded.FindBySource(AlbumSourceID("10001", "album1", "slocA"))
	if !ok || !e.OriginVerified {
		t.Fatalf("在线条目应已验证: %+v", e)
	}
	if _, ok := reloaded.FindBySource(LocalSourceID("旅行/IMG_20200101_a.jpg")); ok {
		t.Fatal("同路径 local 基线条目应被在线身份替换")
	}
	if ve, ok := reloaded.FindBySource(AlbumSourceID("10001", "album1", "slocB")); !ok || ve.MediaKind != MediaKindVideo || ve.RelPath != "旅行/2020/01/VID_b.mp4" {
		t.Fatalf("视频（含 timeline 子目录）登记错误: %+v", ve)
	}
}

func TestSpiderCommitAlbumCanceledKeepsPreviousLedger(t *testing.T) {
	s, root := newTestSpiderLedger(t, LedgerKindAlbum, "")
	albumPath := filepath.Join(root, "相册")
	s.ledger.Upsert(LedgerEntry{SourceID: "album:10001:a:1", RelPath: "相册/old.jpg", SHA256: "old"})
	if err := s.ledger.Save(); err != nil {
		t.Fatal(err)
	}
	oldData, err := os.ReadFile(LedgerPath(root))
	if err != nil {
		t.Fatal(err)
	}

	writeMedia(t, root, "相册/half.jpg", "half")
	s.stageCompletedMedia(AlbumSourceID("10001", "a", "half"), filepath.Join(root, "相册/half.jpg"))
	s.commitAlbumLedger(albumPath, false, false) // 取消：非干净边界

	newData, err := os.ReadFile(LedgerPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if string(oldData) != string(newData) {
		t.Fatal("取消时账本文件必须保持为上一版")
	}
	reloaded, _ := LoadLedger(root)
	if reloaded.Count() != 1 {
		t.Fatalf("取消提交不得追加条目，实际 %d", reloaded.Count())
	}
}

func TestGroupSpiderLedgerIdentityAndRoot(t *testing.T) {
	s, root := newTestSpiderLedger(t, LedgerKindGroupAlbum, "90001")
	if got := s.ledgerDomainRoot("10001"); got != filepath.Join("storage", "qzone", "10001", "qun", "90001") {
		t.Fatalf("群域根错误: %s", got)
	}
	album := gjson.Parse(`{"id":"albumG"}`)
	if id := s.albumMediaSourceID("10001", album, "slocG"); id != "group:10001:90001:albumG:slocG" {
		t.Fatalf("群身份错误: %s", id)
	}
	writeMedia(t, root, "群相册/IMG_g.jpg", "g")
	s.stageCompletedMedia("group:10001:90001:albumG:slocG", filepath.Join(root, "群相册/IMG_g.jpg"))
	s.commitAlbumLedger(filepath.Join(root, "群相册"), false, true)
	l, err := LoadLedger(root)
	if err != nil || l.Count() != 1 || l.GroupID != "90001" || l.Kind != LedgerKindGroupAlbum {
		t.Fatalf("群账本提交错误: %+v err=%v", l, err)
	}
}

func TestFullBackupWipeDoesNotTouchDomainLedger(t *testing.T) {
	// 非增量备份会 RemoveAll 相册子目录；账本在域根 .integrity/，必须存活。
	s, root := newTestSpiderLedger(t, LedgerKindAlbum, "")
	albumPath := filepath.Join(root, "A")
	writeMedia(t, root, "A/x.jpg", "x")
	s.stageCompletedMedia(AlbumSourceID("10001", "a1", "x"), filepath.Join(root, "A/x.jpg"))
	s.commitAlbumLedger(albumPath, false, true)
	if _, err := os.Stat(LedgerPath(root)); err != nil {
		t.Fatalf("提交后账本应存在: %v", err)
	}

	// 模拟 buildLocalFileIndex(albumPath, false) 的清空相册目录行为。
	if err := os.RemoveAll(albumPath); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLedger(root); err != nil {
		t.Fatalf("清空相册目录不得损坏域根账本: %v", err)
	}
}

func TestStageIgnoresPathOutsideRoot(t *testing.T) {
	s, _ := newTestSpiderLedger(t, LedgerKindAlbum, "")
	outside := filepath.Join(t.TempDir(), "escape.jpg")
	if err := os.WriteFile(outside, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	s.stageCompletedMedia(AlbumSourceID("10001", "a", "e"), outside)
	if len(s.staged) != 0 {
		t.Fatalf("域根之外的文件不得暂存: %+v", s.staged)
	}
}
