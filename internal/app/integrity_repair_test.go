// 定点修复测试：计划生成/持久化、隔离范围、原链路复用（注入假执行器）、
// 健康与未登记文件零接触、失败保留可续作、不可自动修复项跳过。

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// fakeRepairRunner 模拟在线链路：对 succeedRels 写入正确文件并登记 verified 条目；
// failRels 进入失败结果；albumErr 模拟网络/登录级失败。
type fakeRepairRunner struct {
	root           string
	kind           LedgerKind
	owner, groupID string
	succeedRels    []string
	failRels       []string
	albumErr       error
	moodErr        error

	gotSelectors []string
	gotMoodItems []FailedItem
	gotBoardItems []FailedItem
	calls        int
}

func (r *fakeRepairRunner) simulateSuccess(rels []string) {
	l, err := LoadLedger(r.root)
	if err != nil {
		panic(err)
	}
	for _, rel := range rels {
		abs := filepath.Join(r.root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), os.ModePerm); err != nil {
			panic(err)
		}
		content := "repaired:" + rel
		if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
			panic(err)
		}
		e, err := EntryFromFile(r.root, rel, "album:"+r.owner+":aid:"+filepath.Base(rel), true)
		if r.kind == LedgerKindShuoshuo || r.kind == LedgerKindBoard {
			prefix := sourcePrefixShuo
			if r.kind == LedgerKindBoard {
				prefix = sourcePrefixBoard
			}
			e.SourceID = MoodMediaSourceID(prefix, r.owner, "tid", "mid-"+filepath.Base(rel), "")
		}
		if err != nil {
			panic(err)
		}
		l.Upsert(e)
	}
	if err := l.Save(); err != nil {
		panic(err)
	}
}

// failResult 构造含失败项的链路结果。
func (r *fakeRepairRunner) failResult() *DownloadResult {
	res := &DownloadResult{}
	for _, rel := range r.failRels {
		res.addFailedItem(FailedItem{Name: rel, Error: "simulated chain failure"})
	}
	return res
}

func (r *fakeRepairRunner) RunAlbumRepair(ctx context.Context, kind LedgerKind, owner, groupID string, selectors []string) (*DownloadResult, error) {
	r.calls++
	r.kind, r.owner, r.groupID, r.gotSelectors = kind, owner, groupID, selectors
	if r.albumErr != nil {
		return nil, r.albumErr
	}
	r.simulateSuccess(r.succeedRels)
	return r.failResult(), nil
}

func (r *fakeRepairRunner) RunMoodRepair(ctx context.Context, owner string, items []FailedItem) (*DownloadResult, error) {
	r.calls++
	r.owner, r.gotMoodItems = owner, items
	if r.moodErr != nil {
		return nil, r.moodErr
	}
	r.simulateSuccess(r.succeedRels)
	return r.failResult(), nil
}

func (r *fakeRepairRunner) RunBoardRepair(ctx context.Context, owner string, items []FailedItem) (*DownloadResult, error) {
	r.calls++
	r.owner, r.gotBoardItems = owner, items
	if r.moodErr != nil {
		return nil, r.moodErr
	}
	r.simulateSuccess(r.succeedRels)
	return r.failResult(), nil
}

func buildAlbumRepairFixture(t *testing.T) (string, *VerifyReport, LedgerDomain, string, string, string) {
	t.Helper()
	root := t.TempDir()
	// 健康文件：修复中不得被写动。
	hSize, hSum := writeMedia(t, root, "A/keep.jpg", "keep-bytes")
	// 变更：将被隔离重建。
	writeMedia(t, root, "A/bad.jpg", "corrupt")
	// 未完成：目标 + sidecar。
	writeMedia(t, root, "A/half.mp4", "half")
	writeMedia(t, root, "A/half.mp4.resume.json", "{}")
	// 未登记：修复不得触碰。
	writeMedia(t, root, "A/extra.jpg", "extra")

	l := NewLedger(LedgerKindAlbum, "10001", "", root)
	l.Upsert(LedgerEntry{SourceID: AlbumSourceID("10001", "aid1", "keep"), RelPath: "A/keep.jpg", Size: hSize, SHA256: hSum, OriginVerified: true, MediaKind: MediaKindImage})
	l.Upsert(LedgerEntry{SourceID: AlbumSourceID("10001", "aid1", "bad"), RelPath: "A/bad.jpg", Size: 999, SHA256: "deadbeef", OriginVerified: true, MediaKind: MediaKindImage})
	l.Upsert(LedgerEntry{SourceID: AlbumSourceID("10001", "aid1", "gone"), RelPath: "A/gone.jpg", Size: 5, SHA256: "beefdead", OriginVerified: true, MediaKind: MediaKindImage})
	l.Upsert(LedgerEntry{SourceID: AlbumSourceID("10001", "aid1", "half"), RelPath: "A/half.mp4", Size: 10, SHA256: "zz", OriginVerified: true, MediaKind: MediaKindVideo})
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}

	report, err := VerifyRoot(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	domain := LedgerDomain{Kind: LedgerKindAlbum, Root: root, OwnerUin: "10001", HasLedger: true}
	return root, report, domain, "A/keep.jpg", "A/extra.jpg", "A/bad.jpg"
}

func TestRepairAlbumQuarantineRebuildAndHealthyUntouched(t *testing.T) {
	root, report, domain, keepRel, extraRel, badRel := buildAlbumRepairFixture(t)
	plan := NewRepairPlan(domain, report)
	if err := plan.Save(); err != nil {
		t.Fatal(err)
	}
	// 未登记项不得进入计划。
	for _, it := range plan.Items {
		if it.Reason == VerifyUnregistered || it.RelPath == extraRel {
			t.Fatalf("未登记项不得入计划: %+v", it)
		}
		if !it.AutoRepairable || it.AlbumName != "A" {
			t.Fatalf("相册计划项应可修复且带相册名: %+v", it)
		}
	}
	if len(plan.Items) != 3 {
		t.Fatalf("计划应有 3 项（changed/missing/incomplete），实际 %d", len(plan.Items))
	}

	keepPath := filepath.Join(root, filepath.FromSlash(keepRel))
	keepStat, _ := os.Stat(keepPath)
	extraPath := filepath.Join(root, filepath.FromSlash(extraRel))
	extraStat, _ := os.Stat(extraPath)
	beforePlan, _ := os.ReadFile(RepairPlanPath(root))

	runner := &fakeRepairRunner{
		root: root, kind: LedgerKindAlbum, owner: "10001",
		succeedRels: []string{"A/bad.jpg", "A/gone.jpg", "A/half.mp4"},
	}
	outcome, err := ExecuteRepair(context.Background(), domain, plan, runner, zap.NewNop().Sugar())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(outcome.Resolved) != 3 || len(outcome.Remaining) != 0 {
		t.Fatalf("修复裁决错误: resolved=%d remaining=%d", len(outcome.Resolved), len(outcome.Remaining))
	}

	// 白名单同时包含相册名与相册 ID。
	var hasName, hasID bool
	for _, s := range runner.gotSelectors {
		if s == "A" {
			hasName = true
		}
		if s == "aid1" {
			hasID = true
		}
	}
	if !hasName || !hasID {
		t.Fatalf("修复白名单缺相册名/ID: %v", runner.gotSelectors)
	}

	// 健康与未登记文件 mtime 不变（零接触）。
	if st, _ := os.Stat(keepPath); st.ModTime() != keepStat.ModTime() || st.Size() != keepStat.Size() {
		t.Fatal("健康文件被修复过程写动")
	}
	if st, _ := os.Stat(extraPath); st.ModTime() != extraStat.ModTime() {
		t.Fatal("未登记文件被修复过程写动")
	}
	// 变更/未完成文件进隔离区且保留相对结构。
	if _, err := os.Stat(filepath.Join(root, ".integrity", "quarantine")) ; err != nil {
		t.Fatal(err)
	}
	quarantineHas := func(suffix string) bool {
		found := false
		_ = filepath.Walk(filepath.Join(root, ".integrity", "quarantine"), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && filepath.ToSlash(path) != "" {
				if strings.HasSuffix(filepath.ToSlash(path), suffix) {
					found = true
				}
			}
			return nil
		})
		return found
	}
	if !quarantineHas("A/bad.jpg") || !quarantineHas("A/half.mp4.resume.json") {
		t.Fatal("隔离区缺少变更文件或 sidecar 标记")
	}
	// 原位损坏文件已移走。
	if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(badRel))); err == nil && st.Size() == int64(len("corrupt")) {
		t.Fatal("原位损坏文件应已被隔离，而不是留在原地")
	}
	// 全部解决：计划文件删除。
	if _, err := os.ReadFile(RepairPlanPath(root)); !os.IsNotExist(err) {
		t.Fatal("全部修复后计划文件应移除")
	}
	_ = beforePlan
}

func TestRepairFailureKeepsPlanAndContinues(t *testing.T) {
	root, report, domain, _, _, _ := buildAlbumRepairFixture(t)
	plan := NewRepairPlan(domain, report)
	if err := plan.Save(); err != nil {
		t.Fatal(err)
	}
	originalPlan, _ := os.ReadFile(RepairPlanPath(root))

	// 第一轮：gone 失败，其余成功。
	runner := &fakeRepairRunner{
		root: root, kind: LedgerKindAlbum, owner: "10001",
		succeedRels: []string{"A/bad.jpg", "A/half.mp4"},
		failRels:    []string{"A/gone.jpg"},
	}
	outcome, err := ExecuteRepair(context.Background(), domain, plan, runner, zap.NewNop().Sugar())
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Resolved) != 2 || len(outcome.Remaining) != 1 {
		t.Fatalf("首轮应 2 成功 1 残留，实际 %d/%d", len(outcome.Resolved), len(outcome.Remaining))
	}
	if outcome.Remaining[0].LastError == "" {
		t.Fatal("残留项应记录失败原因")
	}
	persisted, err := LoadRepairPlan(root)
	if err != nil || len(persisted.Items) != 1 || persisted.Items[0].RelPath != "A/gone.jpg" {
		t.Fatalf("失败计划必须持久化供下次继续: %+v err=%v", persisted, err)
	}

	// 链路级异常（断网/无凭证）：计划文件保持原样。
	plan2, _ := LoadRepairPlan(root)
	runner2 := &fakeRepairRunner{root: root, kind: LedgerKindAlbum, owner: "10001", albumErr: errors.New("network down")}
	if _, err := ExecuteRepair(context.Background(), domain, plan2, runner2, zap.NewNop().Sugar()); err == nil {
		t.Fatal("链路异常应向上返回")
	}
	afterErr, _ := os.ReadFile(RepairPlanPath(root))
	if string(afterErr) == "" {
		t.Fatal("异常后计划不得丢失")
	}
	_ = originalPlan

	// 第二轮续作：gone 成功。
	plan3, _ := LoadRepairPlan(root)
	runner3 := &fakeRepairRunner{root: root, kind: LedgerKindAlbum, owner: "10001", succeedRels: []string{"A/gone.jpg"}}
	outcome3, err := ExecuteRepair(context.Background(), domain, plan3, runner3, zap.NewNop().Sugar())
	if err != nil || len(outcome3.Resolved) != 1 || len(outcome3.Remaining) != 0 {
		t.Fatalf("续作应 1 成功 0 残留: %+v err=%v", outcome3, err)
	}
	if _, err := os.ReadFile(RepairPlanPath(root)); !os.IsNotExist(err) {
		t.Fatal("续作全部成功后计划应移除")
	}
}

func TestRepairMoodReusesRetryChainAndSkipsUnrepairable(t *testing.T) {
	root := t.TempDir()
	// backup.json 里有一条可定位媒体。
	backup := &MoodBackupFile{
		UIN: "10001",
		Posts: []MoodPost{{
			TID: "tid1",
			Media: []MoodMedia{{
				ID: "mid1", Type: "image", URL: "https://x/1.jpg",
				Path: "media/2020/01/a.jpg",
			}},
		}},
	}
	if err := saveMoodBackup(root, backup); err != nil {
		t.Fatal(err)
	}
	mSize, mSum := writeMedia(t, root, "media/2020/02/ok.jpg", "ok")
	// 损坏的已登记媒体 + 无身份孤立半成品（backup.json 里找不到）。
	writeMedia(t, root, "media/2020/01/a.jpg", "bad-bytes")
	writeMedia(t, root, "media/2019/12/orphan.mp4.part", "seg0")

	l := NewLedger(LedgerKindShuoshuo, "10001", "", root)
	l.Upsert(LedgerEntry{SourceID: ShuoshuoSourceID("10001", "tid1", "mid1", ""), RelPath: "media/2020/01/a.jpg", Size: 100, SHA256: "xx", OriginVerified: true, MediaKind: MediaKindImage})
	l.Upsert(LedgerEntry{SourceID: ShuoshuoSourceID("10001", "tidok", "midok", ""), RelPath: "media/2020/02/ok.jpg", Size: mSize, SHA256: mSum, OriginVerified: true, MediaKind: MediaKindImage})
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}

	report, err := VerifyRoot(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	domain := LedgerDomain{Kind: LedgerKindShuoshuo, Root: root, OwnerUin: "10001", HasLedger: true}
	plan := NewRepairPlan(domain, report)

	var repairable, unrepairable []RepairItem
	for _, it := range plan.Items {
		if it.AutoRepairable {
			repairable = append(repairable, it)
		} else {
			unrepairable = append(unrepairable, it)
		}
	}
	if len(repairable) != 1 || repairable[0].RelPath != "media/2020/01/a.jpg" {
		t.Fatalf("可修复项错误: %+v", repairable)
	}
	if repairable[0].TID != "tid1" || repairable[0].MediaID != "mid1" || repairable[0].MediaURL != "https://x/1.jpg" {
		t.Fatalf("说说取源提示补全错误: %+v", repairable[0])
	}
	if len(unrepairable) != 1 {
		t.Fatalf("孤立 part 应不可自动修复: %+v", unrepairable)
	}

	runner := &fakeRepairRunner{root: root, kind: LedgerKindShuoshuo, owner: "10001", succeedRels: []string{"media/2020/01/a.jpg"}}
	outcome, err := ExecuteRepair(context.Background(), domain, plan, runner, zap.NewNop().Sugar())
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.gotMoodItems) != 1 || runner.gotMoodItems[0].Kind != FailedKindShuoShuo || runner.gotMoodItems[0].MoodTID != "tid1" {
		t.Fatalf("必须复用 RetryFailed FailedItem 链路: %+v", runner.gotMoodItems)
	}
	if len(outcome.Resolved) != 1 || len(outcome.Unrepairable) != 1 {
		t.Fatalf("裁决错误: %+v", outcome)
	}
	// 不可修复项保留在计划中。
	persisted, err := LoadRepairPlan(root)
	if err != nil || len(persisted.Items) != 1 || persisted.Items[0].AutoRepairable {
		t.Fatalf("不可修复项应保留待人工处理: %+v err=%v", persisted, err)
	}
}

func TestRepairPlanPersistenceRoundTrip(t *testing.T) {
	root := t.TempDir()
	if HasPendingRepair(root) {
		t.Fatal("新目录不应有计划")
	}
	p := &RepairPlan{Kind: LedgerKindBoard, OwnerUin: "10001", root: root,
		Items: []RepairItem{{RelPath: "media/a.jpg", Reason: RepairReasonMissing, AutoRepairable: true, CreatedAt: time.Now()}}}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	if !HasPendingRepair(root) {
		t.Fatal("保存后应能发现待修复计划")
	}
	got, err := LoadRepairPlan(root)
	if err != nil || len(got.Items) != 1 || got.Kind != LedgerKindBoard {
		t.Fatalf("计划往返错误: %+v err=%v", got, err)
	}
	got.Items = nil
	if err := got.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRepairPlan(root); err != ErrRepairPlanMissing {
		t.Fatalf("空计划保存后应为 missing，实际 %v", err)
	}
}
