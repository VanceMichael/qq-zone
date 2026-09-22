// 离线完整性核验：无登录、无网络、不修改任何媒体文件或账本。
// 输出四类稳定结果（缺失/内容变更/未登记/未完成）与健康统计（已验证/未验证）。

package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/qinjintian/qq-zone/internal/pkg/util"
)

// 核验结果状态。
const (
	VerifyHealthy      = "healthy"      // 账本条目与磁盘文件大小、摘要完全一致
	VerifyMissing      = "missing"      // 账本登记的文件不存在
	VerifyChanged      = "changed"      // 文件存在但字节数或摘要不一致
	VerifyIncomplete   = "incomplete"   // 续传半成品（.resume.json）或 HLS 中间产物（.part）
	VerifyUnregistered = "unregistered" // 在管理范围内但账本未登记
)

// 未完成标记后缀。
const (
	resumeSidecarExt = ".resume.json"
	hlsPartExt2      = ".part"
)

// albumMetadataFile 是相册目录里导出的元数据，不属于核心媒体。
const albumMetadataFile = "album_metadata.json"

// VerifyItem 是一条核验明细。
type VerifyItem struct {
	Status         string `json:"status"`
	RelPath        string `json:"rel_path"`
	SourceID       string `json:"source_id,omitempty"`
	MediaKind      string `json:"media_kind,omitempty"`
	OriginVerified bool   `json:"origin_verified,omitempty"`
	WantSize       int64  `json:"want_size,omitempty"`
	GotSize        int64  `json:"got_size,omitempty"`
	WantSHA256     string `json:"want_sha256,omitempty"`
	GotSHA256      string `json:"got_sha256,omitempty"`
	Detail         string `json:"detail,omitempty"` // 未完成时说明标记文件等
}

// VerifyReport 是一次核验的完整、可复现结果。
type VerifyReport struct {
	Kind              LedgerKind   `json:"kind"`
	Root              string       `json:"root"`
	HasLedger         bool         `json:"has_ledger"`
	HealthyVerified   int          `json:"healthy_verified"`
	HealthyUnverified int          `json:"healthy_unverified"`
	Items             []VerifyItem `json:"items"`
}

// Section 返回某一状态的明细（结果按 rel_path 稳定排序）。
func (r *VerifyReport) Section(status string) []VerifyItem {
	if r == nil {
		return nil
	}
	out := make([]VerifyItem, 0)
	for _, it := range r.Items {
		if it.Status == status {
			out = append(out, it)
		}
	}
	return out
}

// ProblemCount 返回除健康外四类问题总数。
func (r *VerifyReport) ProblemCount() int {
	if r == nil {
		return 0
	}
	return len(r.Section(VerifyMissing)) + len(r.Section(VerifyChanged)) +
		len(r.Section(VerifyIncomplete)) + len(r.Section(VerifyUnregistered))
}

// VerifyProgress 在哈希扫描过程中汇报进度（已完成文件数/总文件数），可为 nil。
type VerifyProgress func(done, total int, currentRel string)

// VerifyRoot 对单个域根做纯本地核验。
// 账本不存在时返回 ErrLedgerMissing（调用方可引导用户建立基线或仅扫描）；不做任何写操作。
func VerifyRoot(root string, progress VerifyProgress) (*VerifyReport, error) {
	l, err := LoadLedger(root)
	if err != nil {
		return nil, err
	}
	return verifyWithLedger(root, l, progress), nil
}

// ScanRootWithoutLedger 用于没有账本的旧备份：只列管理范围内的现存媒体（全部算未登记）与未完成项。
// 同样严格只读，不需要也不创建账本。
func ScanRootWithoutLedger(kind LedgerKind, root string, progress VerifyProgress) *VerifyReport {
	return verifyWithLedger(root, NewLedger(kind, "", "", root), progress)
}

// verifyWithLedger 执行核验；l 为空账本时所有现存媒体都会落入未登记。
func verifyWithLedger(root string, l *Ledger, progress VerifyProgress) *VerifyReport {
	mediaFiles, markers := scanManagedFiles(l.Kind, root)

	report := &VerifyReport{Kind: l.Kind, Root: root, HasLedger: l.Count() > 0 || ledgerExists(root)}
	consumed := make(map[string]bool) // 已被账本条目或未完成标记消费的磁盘文件
	markerTargets := make(map[string]string)
	for _, mk := range markers {
		if prev, exists := markerTargets[mk.targetRel]; !exists || len(mk.markerRel) < len(prev) {
			markerTargets[mk.targetRel] = mk.markerRel
		}
	}

	total := len(l.Entries) + len(mediaFiles)
	done := 0
	tick := func(rel string) {
		done++
		if progress != nil {
			progress(done, total, rel)
		}
	}

	// 1) 逐条核对账本条目：缺失 → 未完成 → 大小 → 摘要 → 健康（顺序与 FR-5 一致）。
	entries := append([]LedgerEntry(nil), l.Entries...)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].RelPath < entries[j].RelPath })
	for _, e := range entries {
		abs := filepath.Join(root, filepath.FromSlash(e.RelPath))
		fi, statErr := os.Stat(abs)
		if statErr != nil || !fi.Mode().IsRegular() {
			report.Items = append(report.Items, VerifyItem{
				Status: VerifyMissing, RelPath: e.RelPath, SourceID: e.SourceID,
				MediaKind: e.MediaKind, OriginVerified: e.OriginVerified, WantSize: e.Size,
			})
			tick(e.RelPath)
			continue
		}

		if markerRel, incomplete := markerTargets[e.RelPath]; incomplete {
			report.Items = append(report.Items, VerifyItem{
				Status: VerifyIncomplete, RelPath: e.RelPath, SourceID: e.SourceID,
				MediaKind: e.MediaKind, OriginVerified: e.OriginVerified,
				GotSize: fi.Size(), Detail: "存在未完成标记: " + markerRel,
			})
			consumed[e.RelPath] = true
			tick(e.RelPath)
			continue
		}

		if fi.Size() != e.Size {
			report.Items = append(report.Items, VerifyItem{
				Status: VerifyChanged, RelPath: e.RelPath, SourceID: e.SourceID,
				MediaKind: e.MediaKind, OriginVerified: e.OriginVerified,
				WantSize: e.Size, GotSize: fi.Size(), WantSHA256: e.SHA256,
				Detail: "字节数不一致",
			})
			consumed[e.RelPath] = true
			tick(e.RelPath)
			continue
		}

		gotSHA, hashErr := util.FileSHA256(abs)
		if hashErr != nil || gotSHA != e.SHA256 {
			item := VerifyItem{
				Status: VerifyChanged, RelPath: e.RelPath, SourceID: e.SourceID,
				MediaKind: e.MediaKind, OriginVerified: e.OriginVerified,
				WantSize: e.Size, GotSize: fi.Size(), WantSHA256: e.SHA256,
				GotSHA256: gotSHA, Detail: "内容摘要不一致",
			}
			if hashErr != nil {
				item.Detail = "无法读取文件: " + hashErr.Error()
			}
			report.Items = append(report.Items, item)
			consumed[e.RelPath] = true
			tick(e.RelPath)
			continue
		}

		if e.OriginVerified {
			report.HealthyVerified++
		} else {
			report.HealthyUnverified++
		}
		consumed[e.RelPath] = true
		tick(e.RelPath)
	}

	// 2) 未完成标记：目标存在但没有账本条目 → 未完成（连目标一起消费，避免再报未登记）；
	//    目标不存在也没有账本条目 → 孤立标记，未完成。
	entryByRel := make(map[string]LedgerEntry, len(entries))
	for _, e := range entries {
		entryByRel[e.RelPath] = e
	}
	markerSeen := make(map[string]bool)
	for _, mk := range markers {
		if markerSeen[mk.targetRel] {
			continue
		}
		markerSeen[mk.targetRel] = true
		if _, hasEntry := entryByRel[mk.targetRel]; hasEntry {
			continue // 条目循环已按优先级处理
		}
		item := VerifyItem{
			Status: VerifyIncomplete, RelPath: mk.targetRel, Detail: "存在未完成标记: " + mk.markerRel,
			MediaKind: MediaKindFromExt(mk.targetRel),
		}
		if _, targetExists := mediaFiles[mk.targetRel]; targetExists {
			consumed[mk.targetRel] = true
		} else {
			// 目标都不在了，直接把标记文件本身作为未完成明细。
			item.RelPath = mk.markerRel
		}
		report.Items = append(report.Items, item)
	}

	// 3) 其余在管理范围内但账本没有的媒体文件 → 未登记。
	rels := make([]string, 0, len(mediaFiles))
	for rel := range mediaFiles {
		if consumed[rel] {
			continue
		}
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		report.Items = append(report.Items, VerifyItem{
			Status: VerifyUnregistered, RelPath: rel,
			MediaKind: MediaKindFromExt(rel), GotSize: mediaFiles[rel],
		})
	}

	sort.SliceStable(report.Items, func(i, j int) bool {
		if report.Items[i].Status != report.Items[j].Status {
			return report.Items[i].Status < report.Items[j].Status
		}
		return report.Items[i].RelPath < report.Items[j].RelPath
	})
	return report
}

// diskMarker 记录一个未完成标记文件及其对应的目标媒体 rel_path。
type diskMarker struct {
	markerRel string
	targetRel string
}

// scanManagedFiles 只做文件系统读操作：返回管理范围内的候选媒体（rel→size）与未完成标记。
// 相册/群相册：递归整个域根，排除 .integrity/、album_metadata.json、标记文件；
// 说说/留言板：仅 media/ 目录，排除标记文件。
func scanManagedFiles(kind LedgerKind, root string) (map[string]int64, []diskMarker) {
	media := make(map[string]int64)
	var markers []diskMarker

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 只读核验：遇到无权限/瞬时错误跳过，不中断整次扫描
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "" || strings.HasPrefix(rel, "..") {
			return nil
		}

		// 排除 .integrity/ 下所有内容（账本、修复计划、隔离区）。
		if isUnderIntegrityDir(rel) {
			return nil
		}

		name := d.Name()
		// 标记识别必须先过管理范围：说说/留言板只关心 media/ 内的媒体，
		// avatars 等辅助目录里的 sidecar/.part 既不算未完成也不算未登记（审查 F2）。
		if !inManagedScope(kind, rel, name) {
			return nil
		}
		switch {
		case strings.HasSuffix(name, resumeSidecarExt):
			markers = append(markers, diskMarker{markerRel: rel, targetRel: strings.TrimSuffix(rel, resumeSidecarExt)})
			return nil
		case strings.HasSuffix(name, hlsPartExt2):
			markers = append(markers, diskMarker{markerRel: rel, targetRel: strings.TrimSuffix(rel, hlsPartExt2)})
			return nil
		}

		if fi, statErr := d.Info(); statErr == nil {
			media[rel] = fi.Size()
		}
		return nil
	})
	return media, markers
}

// isUnderIntegrityDir 判断 rel 是否位于域根的 .integrity/ 目录内（兼容多级）。
func isUnderIntegrityDir(rel string) bool {
	first := rel
	if i := strings.Index(rel, "/"); i >= 0 {
		first = rel[:i]
	}
	return first == integrityDirName
}

// inManagedScope 判断文件是否属于核心媒体扫描范围。
func inManagedScope(kind LedgerKind, rel, name string) bool {
	switch kind {
	case LedgerKindShuoshuo, LedgerKindBoard:
		return strings.HasPrefix(rel, "media/")
	default:
		// 相册/群相册：除元数据导出文件外，递归范围内的普通文件都算媒体。
		return name != albumMetadataFile
	}
}

// ledgerExists 仅用 stat 判断账本文件是否存在（读失败一律按不存在处理，避免核验报错中断）。
func ledgerExists(root string) bool {
	_, err := os.Stat(LedgerPath(root))
	return err == nil
}
