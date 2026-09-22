// 说说/留言板账本会话的本地测试：身份、跳过决策、暂存提交与取消语义；
// 以及 downloadOneMedia 在「已有文件」分支上无需网络的跳过行为。

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/qinjintian/qq-zone/internal/pkg/util"
	"go.uber.org/zap"
)

func initMediaSession(t *testing.T, kind LedgerKind, prefix, uin string) (*mediaLedgerSession, string) {
	t.Helper()
	root := t.TempDir()
	s := &mediaLedgerSession{}
	if err := s.init(kind, prefix, uin, root); err != nil {
		t.Fatalf("init session: %v", err)
	}
	return s, root
}

func TestMediaLedgerSessionIdentityFallback(t *testing.T) {
	s, _ := initMediaSession(t, LedgerKindShuoshuo, sourcePrefixShuo, "10001")
	m := &MoodMedia{ID: "pic9", URL: "http://x/1"}
	if got := s.sourceID("tid1", m); got != "shuoshuo:10001:tid1:pic9" {
		t.Fatalf("媒体 ID 身份错误: %s", got)
	}
	m2 := &MoodMedia{ID: "", URL: "http://x/2"}
	want := "shuoshuo:10001:tid1:url:" + util.MD5("http://x/2")
	if got := s.sourceID("tid1", m2); got != want {
		t.Fatalf("URL 兜底身份错误: %s, want %s", got, want)
	}
}

func TestMediaLedgerSessionDecisionAndStage(t *testing.T) {
	s, root := initMediaSession(t, LedgerKindShuoshuo, sourcePrefixShuo, "10001")
	rel := "media/2021/03/IMG_a.jpg"
	size, sum := writeMedia(t, root, rel, "mood-pic")
	m := &MoodMedia{ID: "mid1", URL: "http://x/a", Type: "image", Path: rel}
	sid := s.sourceID("tid1", m)
	s.ledger.Upsert(LedgerEntry{SourceID: sid, RelPath: rel, Size: size, SHA256: sum, OriginVerified: true, MediaKind: MediaKindImage})

	if d := s.decision("tid1", m); d != LedgerSkipHealthy {
		t.Fatalf("一致应 healthy，实际 %d", d)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte("changed!"), 0644); err != nil {
		t.Fatal(err)
	}
	if d := s.decision("tid1", m); d != LedgerSkipMismatch {
		t.Fatalf("改动应 mismatch，实际 %d", d)
	}
	if d := s.decision("tid-other", &MoodMedia{ID: "x", URL: "u"}); d != LedgerSkipNoEntry {
		t.Fatalf("无身份应 noEntry，实际 %d", d)
	}

	// 恢复内容后 stage + commit。
	writeMedia(t, root, rel, "mood-pic")
	voiceRel := "media/2021/04/AUD_v.mp3"
	writeMedia(t, root, voiceRel, "voice-bytes")
	mv := &MoodMedia{ID: "vid", URL: "http://x/v", Type: "voice", Path: voiceRel}
	s.stage("tid1", m)
	s.stage("tid2", mv)
	s.commit(true, zap.NewNop().Sugar())

	l, err := LoadLedger(root)
	if err != nil || l.Count() != 2 {
		t.Fatalf("提交后条目数错误: %v err=%v", l, err)
	}
	if e, ok := l.FindBySource(s.sourceID("tid2", mv)); !ok || !e.OriginVerified || e.MediaKind != MediaKindVoice || e.RelPath != voiceRel {
		t.Fatalf("语音条目错误: %+v", e)
	}
}

func TestMediaLedgerSessionCancelDiscardsStaged(t *testing.T) {
	s, root := initMediaSession(t, LedgerKindBoard, sourcePrefixBoard, "10001")
	rel := "media/2021/03/a.jpg"
	writeMedia(t, root, rel, "board")
	s.ledger.Upsert(LedgerEntry{SourceID: "board:10001:old:1", RelPath: "media/old.jpg", SHA256: "z"})
	if err := s.ledger.Save(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(LedgerPath(root))

	s.stage("tid", &MoodMedia{ID: "m", URL: "u", Path: rel})
	s.commit(false, zap.NewNop().Sugar()) // 取消

	after, _ := os.ReadFile(LedgerPath(root))
	if string(before) != string(after) {
		t.Fatal("取消提交必须保留旧账本字节")
	}
	l, _ := LoadLedger(root)
	if l.Count() != 1 {
		t.Fatalf("取消后不应有新条目，实际 %d", l.Count())
	}

	// 域外路径拒绝暂存。
	s.stage("tid", &MoodMedia{ID: "m2", URL: "u", Path: "../escape.jpg"})
	s.commit(true, zap.NewNop().Sugar())
	l, _ = LoadLedger(root)
	if l.Count() != 1 {
		t.Fatalf("域外路径不得登记，实际 %d", l.Count())
	}
}

func TestMoodDownloadOneMediaLedgerSkipBranches(t *testing.T) {
	root := t.TempDir()
	b := &MoodBackup{logger: zap.NewNop().Sugar()}
	if err := b.ledger.init(LedgerKindShuoshuo, sourcePrefixShuo, "10001", root); err != nil {
		t.Fatal(err)
	}
	rel := "media/2022/01/IMG_1.jpg"
	size, sum := writeMedia(t, root, rel, "pic")
	m := &MoodMedia{ID: "mid", URL: "http://x/1", Type: "image", Path: rel}
	posts := []MoodPost{{TID: "tid1", Media: []MoodMedia{*m}}}
	mm := &posts[0].Media[0]

	// 无条目：沿用旧「非空即跳过」，不得触网（client 为 nil，走到网络必然 panic），且不登记。
	b.downloadOneMedia(context.Background(), root, "10001", posts, 0, mm)
	if b.ledger.ledger.Count() != 0 {
		t.Fatal("旧行为跳过不应写账本")
	}
	if b.results.ImageCount != 1 {
		t.Fatalf("无条目跳过应计入图片成功数，实际 %d", b.results.ImageCount)
	}

	// 写入一致账本条目：仍离线跳过（同样不触网）。
	b.ledger.ledger.Upsert(LedgerEntry{
		SourceID: b.ledger.sourceID("tid1", mm), RelPath: rel, Size: size, SHA256: sum,
		OriginVerified: true, MediaKind: MediaKindImage,
	})
	b.downloadOneMedia(context.Background(), root, "10001", posts, 0, mm)
	if b.results.ImageCount != 2 {
		t.Fatalf("账本一致应离线跳过，ImageCount=%d", b.results.ImageCount)
	}
}

func TestBoardDownloadOneMediaLegacySkipNoNetwork(t *testing.T) {
	root := t.TempDir()
	b := &BoardBackup{logger: zap.NewNop().Sugar()}
	if err := b.ledger.init(LedgerKindBoard, sourcePrefixBoard, "10001", root); err != nil {
		t.Fatal(err)
	}
	rel := "media/2022/01/IMG_b.jpg"
	writeMedia(t, root, rel, "board-pic")
	m := &MoodMedia{ID: "mid", URL: "https://x/1.jpg", Type: "image", Path: rel}
	posts := []MoodPost{{TID: "tid1", Media: []MoodMedia{*m}}}
	// 无账本条目 + 非空文件：旧行为直接跳过（client 为 nil，若触网即 panic）。
	b.downloadOneMedia(context.Background(), root, "10001", posts, 0, &posts[0].Media[0])
	if b.ledger.ledger.Count() != 0 || b.results.ImageCount != 1 {
		t.Fatalf("留言板旧跳过行为异常: 条目=%d image=%d", b.ledger.ledger.Count(), b.results.ImageCount)
	}
}
