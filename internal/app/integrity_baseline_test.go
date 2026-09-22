// 本地基线与「纳入未登记」测试：来源未验证、未完成排除、已有账本拒绝、纳入不改文件。

package app

import (
	"errors"
	"testing"
)

func TestEstablishBaselineMarksUnverified(t *testing.T) {
	root := t.TempDir()
	writeMedia(t, root, "A/ok.jpg", "ok")
	writeMedia(t, root, "A/2021/03/v.mp4", "video")
	// 续传半成品：目标 + sidecar，都不能进基线。
	writeMedia(t, root, "A/half.mp4", "half")
	writeMedia(t, root, "A/half.mp4.resume.json", "{}")
	// 孤立 HLS .part。
	writeMedia(t, root, "A/ghost.mp4.part", "seg")
	// 非媒体元数据不纳入。
	writeMedia(t, root, "A/album_metadata.json", "{}")

	l, res, err := EstablishBaseline(LedgerKindAlbum, "10001", "", root, nil)
	if err != nil {
		t.Fatalf("establish: %v", err)
	}
	if res.EntryCount != 2 {
		t.Fatalf("基线条目数 = %d, want 2", res.EntryCount)
	}
	for _, e := range l.Entries {
		if e.OriginVerified || !IsLocalSource(e.SourceID) {
			t.Fatalf("基线条目必须来源未验证且 local 身份: %+v", e)
		}
	}
	if e, ok := l.FindByRelPath("A/2021/03/v.mp4"); !ok || e.MediaKind != MediaKindVideo {
		t.Fatalf("视频基线条目错误: %+v", e)
	}
	if len(res.IncompleteList) != 3 {
		t.Fatalf("未完成排除清单 = %v，want half 目标+sidecar+part", res.IncompleteList)
	}

	// 已有账本拒绝重建。
	if _, _, err := EstablishBaseline(LedgerKindAlbum, "10001", "", root, nil); !errors.Is(err, ErrLedgerExists) {
		t.Fatalf("重复建基线应 ErrLedgerExists，实际 %v", err)
	}

	// 基线后立即核验：两条健康（未验证），无缺失/变更。
	r, err := VerifyRoot(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.HealthyUnverified != 2 || r.HealthyVerified != 0 {
		t.Fatalf("基线后核验健康统计错误: %+v", r)
	}
	if len(r.Section(VerifyMissing)) != 0 || len(r.Section(VerifyChanged)) != 0 {
		t.Fatalf("基线条目不应报缺失/变更: %+v", r.Items)
	}
}

func TestAdoptUnregisteredAddsOnlyThem(t *testing.T) {
	root := t.TempDir()
	// 先建账本并登记 a。
	l0, _, err := EstablishBaseline(LedgerKindShuoshuo, "10001", "", root, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = l0

	// 新出现两个未登记文件 + 一个未完成 part。
	writeMedia(t, root, "media/2022/01/new1.jpg", "n1")
	writeMedia(t, root, "media/2022/02/new2.jpg", "n2")
	writeMedia(t, root, "media/2022/02/bad.mp4.part", "p")

	// 基线时 media/ 为空，账本为空；新文件出现后应全部报未登记。
	before := treeDigest(t, root)
	r, err := VerifyRoot(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	added, err := AdoptUnregistered(root, r, nil)
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if added != 2 {
		t.Fatalf("纳入数 = %d, want 2（part 不算）", added)
	}
	after := treeDigest(t, root)
	// 允许 .integrity/ledger.json 变化（账本本就要更新）；其余文件必须逐字节不变。
	delete(before, ".integrity/ledger.json")
	delete(before, ".integrity/ledger.bak")
	delete(after, ".integrity/ledger.json")
	delete(after, ".integrity/ledger.bak")
	if !mapDigestEqual(before, after) {
		t.Fatal("纳入基线不得改动任何媒体文件")
	}

	l, err := LoadLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"media/2022/01/new1.jpg", "media/2022/02/new2.jpg"} {
		e, ok := l.FindByRelPath(rel)
		if !ok || e.OriginVerified || !IsLocalSource(e.SourceID) {
			t.Fatalf("纳入条目必须 local/未验证: %+v", e)
		}
	}
	// 幂等：再次核验后未登记为 0，再纳入 0 条。
	r2, err := VerifyRoot(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.Section(VerifyUnregistered)) != 0 {
		t.Fatalf("纳入后不应再有未登记: %+v", r2.Section(VerifyUnregistered))
	}
	added2, _ := AdoptUnregistered(root, r2, nil)
	if added2 != 0 {
		t.Fatalf("重复纳入应为 0，实际 %d", added2)
	}
}

func mapDigestEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
