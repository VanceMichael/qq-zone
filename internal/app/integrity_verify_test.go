// 离线核验器测试：四类分类正确性、重复运行稳定性、严格只读。

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/qinjintian/qq-zone/internal/pkg/util"
)

// treeDigest 对目录下所有文件做 路径+sha256 快照，用来证明核验零写入。
func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		data, rErr := os.ReadFile(path)
		if rErr != nil {
			return rErr
		}
		sum := sha256.Sum256(data)
		snap[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func saveLedgerForVerify(t *testing.T, root string, entries []LedgerEntry) *Ledger {
	t.Helper()
	ledger := NewLedger(LedgerKindAlbum, "10001", "", root)
	for _, e := range entries {
		ledger.Upsert(e)
	}
	if err := ledger.Save(); err != nil {
		t.Fatal(err)
	}
	return ledger
}

func TestVerifyAlbumFixtureAllCategories(t *testing.T) {
	root := t.TempDir()

	// 健康（已验证）
	hSize, hSum := writeMedia(t, root, "A/ok1.jpg", "healthy-1")
	// 健康（基线未验证）
	uSize, uSum := writeMedia(t, root, "A/ok2.jpg", "healthy-2")
	// 缺失：不写文件
	// 大小不一致
	cSize, cSum := writeMedia(t, root, "A/changed1.jpg", "changed-size")
	// 摘要不一致（大小保持一致）
	changedHash := "same-size"
	writeMedia(t, root, "A/changed2.jpg", changedHash)
	// 未完成：目标文件 + sidecar
	writeMedia(t, root, "A/half.mp4", "half-bytes")
	if err := os.WriteFile(filepath.Join(root, "A/half.mp4.resume.json"), []byte(`{"uri":"x"}`), 0644); err != nil {
		t.Fatal(err)
	}
	// 孤立 HLS 半成品（无最终文件、无条目）
	writeMedia(t, root, "A/ghost.mp4.part", "seg0")
	// 未登记文件
	writeMedia(t, root, "A/extra.jpg", "extra")
	// 应排除：相册元数据
	writeMedia(t, root, "A/album_metadata.json", "{}")

	sameSize := int64(len(changedHash))
	sameSum := sha256Hex(changedHash)
	entries := []LedgerEntry{
		{SourceID: "album:10001:a1:ok1", RelPath: "A/ok1.jpg", Size: hSize, SHA256: hSum, OriginVerified: true, MediaKind: MediaKindImage},
		{SourceID: LocalSourceID("A/ok2.jpg"), RelPath: "A/ok2.jpg", Size: uSize, SHA256: uSum, OriginVerified: false, MediaKind: MediaKindImage},
		{SourceID: "album:10001:a1:gone", RelPath: "A/gone.jpg", Size: 10, SHA256: "x", OriginVerified: true, MediaKind: MediaKindImage},
		{SourceID: "album:10001:a1:c1", RelPath: "A/changed1.jpg", Size: cSize + 5, SHA256: cSum, OriginVerified: true, MediaKind: MediaKindImage},
		{SourceID: "album:10001:a1:c2", RelPath: "A/changed2.jpg", Size: sameSize, SHA256: differentSameLenHex(sameSum), OriginVerified: true, MediaKind: MediaKindImage},
		{SourceID: "album:10001:a1:half", RelPath: "A/half.mp4", Size: 999, SHA256: "z", OriginVerified: true, MediaKind: MediaKindVideo},
	}
	saveLedgerForVerify(t, root, entries)

	before := treeDigest(t, root)
	r1, err := VerifyRoot(root, nil)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	r2, err := VerifyRoot(root, nil)
	if err != nil {
		t.Fatalf("verify2: %v", err)
	}
	after := treeDigest(t, root)

	// TR-5.3：核验零写入。
	if !reflect.DeepEqual(before, after) {
		t.Fatal("核验前后文件树不一致：核验产生了写入")
	}
	// TR-5.2：两次结果完全一致。
	if !reflect.DeepEqual(r1, r2) {
		t.Fatal("两次核验结果必须逐字段一致")
	}

	if n := len(r1.Section(VerifyMissing)); n != 1 || r1.Section(VerifyMissing)[0].RelPath != "A/gone.jpg" {
		t.Fatalf("缺失分类错误: %+v", r1.Section(VerifyMissing))
	}
	changed := r1.Section(VerifyChanged)
	if len(changed) != 2 {
		t.Fatalf("变更应有 2 项（大小+摘要），实际 %d: %+v", len(changed), changed)
	}
	var sawSize, sawHash bool
	for _, c := range changed {
		if c.Detail == "字节数不一致" {
			sawSize = true
		}
		if c.Detail == "内容摘要不一致" {
			sawHash = true
		}
	}
	if !sawSize || !sawHash {
		t.Fatalf("变更明细缺少子类型: size=%v hash=%v", sawSize, sawHash)
	}
	incomp := r1.Section(VerifyIncomplete)
	if len(incomp) != 2 {
		t.Fatalf("未完成应有 2 项（sidecar+孤立 part），实际 %d: %+v", len(incomp), incomp)
	}
	var incompRels []string
	for _, it := range incomp {
		incompRels = append(incompRels, it.RelPath)
	}
	sort.Strings(incompRels)
	if len(incompRels) != 2 || incompRels[0] != "A/ghost.mp4.part" || incompRels[1] != "A/half.mp4" {
		t.Fatalf("未完成明细错误: %v", incompRels)
	}
	unreg := r1.Section(VerifyUnregistered)
	if len(unreg) != 1 || unreg[0].RelPath != "A/extra.jpg" {
		t.Fatalf("未登记分类错误（元数据与 .integrity 必须排除）: %+v", unreg)
	}
	if r1.HealthyVerified != 1 || r1.HealthyUnverified != 1 {
		t.Fatalf("健康统计错误: verified=%d unverified=%d", r1.HealthyVerified, r1.HealthyUnverified)
	}
	if r1.ProblemCount() != 6 {
		t.Fatalf("问题总数 = %d, want 6", r1.ProblemCount())
	}
}

func TestVerifyMoodScopeExcludesAuxiliary(t *testing.T) {
	root := t.TempDir()
	// 媒体范围内 1 个登记健康 + 1 个未登记。
	mSize, mSum := writeMedia(t, root, "media/2020/02/a.jpg", "mood")
	writeMedia(t, root, "media/2020/02/b.jpg", "unreg")
	// 辅助文件一律不算未登记。
	writeMedia(t, root, "avatars/10001.jpg", "face")
	writeMedia(t, root, "assets/viewer.js", "js")
	writeMedia(t, root, "data/backup.json", "{}")
	writeMedia(t, root, "raw/page_1.json", "{}")
	writeMedia(t, root, "index.html", "<html>")

	l := NewLedger(LedgerKindShuoshuo, "10001", "", root)
	l.Upsert(LedgerEntry{
		SourceID: ShuoshuoSourceID("10001", "tid", "mid", ""), RelPath: "media/2020/02/a.jpg",
		Size: mSize, SHA256: mSum, OriginVerified: true, MediaKind: MediaKindImage,
	})
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}

	r, err := VerifyRoot(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != LedgerKindShuoshuo || r.HealthyVerified != 1 {
		t.Fatalf("说说健康统计错误: %+v", r)
	}
	unreg := r.Section(VerifyUnregistered)
	if len(unreg) != 1 || unreg[0].RelPath != "media/2020/02/b.jpg" {
		t.Fatalf("说说只应扫描 media/：%+v", unreg)
	}
	if r.ProblemCount() != 1 {
		t.Fatalf("辅助文件不得制造噪声，问题数 = %d", r.ProblemCount())
	}
}

func TestVerifyMoodIgnoresMarkersOutsideMediaScope(t *testing.T) {
	root := t.TempDir()
	writeMedia(t, root, "media/2020/01/a.jpg", "a")
	// avatars 等辅助目录的残留 sidecar/.part 是真实场景，但按范围约定不得报未完成/未登记。
	writeMedia(t, root, "avatars/10001.jpg", "face")
	writeMedia(t, root, "avatars/10001.jpg.resume.json", "{}")
	writeMedia(t, root, "avatars/x.mp4.part", "seg")
	writeMedia(t, root, "assets/v.js.part", "js")

	l := NewLedger(LedgerKindShuoshuo, "10001", "", root)
	size, sum := digestVerifyMedia(t, root, "media/2020/01/a.jpg")
	l.Upsert(LedgerEntry{SourceID: ShuoshuoSourceID("10001", "t", "m", ""), RelPath: "media/2020/01/a.jpg", Size: size, SHA256: sum, OriginVerified: true, MediaKind: MediaKindImage})
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}

	r, err := VerifyRoot(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.ProblemCount() != 0 {
		t.Fatalf("media/ 之外的标记不得产生任何问题项: %+v", r.Items)
	}
	if r.HealthyVerified != 1 {
		t.Fatalf("仅 media 内 1 个健康文件，实际 verified=%d", r.HealthyVerified)
	}

	// 无账本扫描同样忽略范围外标记；media 内文件按无账本语义报未登记。
	r2 := ScanRootWithoutLedger(LedgerKindShuoshuo, root, nil)
	if len(r2.Section(VerifyIncomplete)) != 0 {
		t.Fatalf("范围外标记不得报未完成: %+v", r2.Section(VerifyIncomplete))
	}
	unreg := r2.Section(VerifyUnregistered)
	if len(unreg) != 1 || unreg[0].RelPath != "media/2020/01/a.jpg" {
		t.Fatalf("仅 media 内 1 个文件应报未登记: %+v", unreg)
	}
}

// digestVerifyMedia 取夹具文件 size+摘要。
func digestVerifyMedia(t *testing.T, root, rel string) (int64, string) {
	t.Helper()
	size, sum, err := util.FileDigest(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return size, sum
}

func TestVerifyMissingLedgerAndScanOnly(t *testing.T) {
	root := t.TempDir()
	if _, err := VerifyRoot(root, nil); err != ErrLedgerMissing {
		t.Fatalf("无账本应返回 ErrLedgerMissing，实际 %v", err)
	}
	writeMedia(t, root, "media/2020/01/x.jpg", "x")
	writeMedia(t, root, "media/2020/01/y.mp4.part", "p")
	r := ScanRootWithoutLedger(LedgerKindBoard, root, nil)
	if r.HasLedger {
		t.Fatal("仅扫描不应声称存在账本")
	}
	if len(r.Section(VerifyUnregistered)) != 1 || len(r.Section(VerifyIncomplete)) != 1 {
		t.Fatalf("无账本扫描分类错误: %+v", r.Items)
	}
}

// sha256Hex 返回内容的小写十六进制 SHA-256。
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// differentSameLenHex 构造与入参等长但确定不同的十六进制串。
func differentSameLenHex(x string) string {
	b := []byte(x)
	for i := range b {
		if b[i] == '0' {
			b[i] = '1'
		} else {
			b[i] = '0'
		}
	}
	return string(b)
}
