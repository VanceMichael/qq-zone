// 旧备份本地基线：在无账本域根显式建立「来源未验证」登记；以及把核验出的未登记媒体纳入基线。
// 基线只描述当前磁盘字节，绝不声称已与空间原件核对。

package app

import (
	"errors"
	"sort"
)

// ErrLedgerExists 表示域根已有账本，不能再「建立基线」覆盖；未登记项请走纳入基线。
var ErrLedgerExists = errors.New("ledger already exists; use 未登记纳入基线 instead")

// BaselineNotice 是所有涉及基线的界面必须向用户明示的语义。
const BaselineNotice = "基线仅记录当前磁盘上的字节（路径/大小/SHA-256），来源未验证，" +
	"并不代表这些文件已与 QQ 空间原件核对；只有以后真正在线重新下载/修复成功的文件才会标记为来源已验证。"

// BaselineResult 汇总一次建基线的成果。
type BaselineResult struct {
	Root           string   `json:"root"`
	Kind           LedgerKind `json:"kind"`
	EntryCount     int      `json:"entry_count"`      // 纳入基线的媒体数
	IncompleteList []string `json:"incomplete_list"`  // 被排除的未完成目标/标记（rel_path）
}

// EstablishBaseline 为没有账本的域根建立本地基线。
// ownerUin/groupID 仅用于账本元信息（群相册传 groupID，其余传空）。
// 未完成文件（.part/.resume.json 对应目标）一律排除并在结果中列出。
func EstablishBaseline(kind LedgerKind, ownerUin, groupID, root string, progress VerifyProgress) (*Ledger, *BaselineResult, error) {
	if _, err := LoadLedger(root); err == nil {
		return nil, nil, ErrLedgerExists
	} else if !errors.Is(err, ErrLedgerMissing) {
		return nil, nil, err
	}

	mediaFiles, markers := scanManagedFiles(kind, root)
	incompleteTargets := map[string]bool{}
	incompleteMarkers := map[string]bool{}
	for _, mk := range markers {
		incompleteMarkers[mk.markerRel] = true
		// 目标存在（普通续传半成品会留下最终文件名）才排除目标；目标缺失时只排除标记本身。
		if _, ok := mediaFiles[mk.targetRel]; ok {
			incompleteTargets[mk.targetRel] = true
		}
	}

	l := NewLedger(kind, ownerUin, groupID, root)
	rels := make([]string, 0, len(mediaFiles))
	for rel := range mediaFiles {
		rels = append(rels, rel)
	}
	sort.Strings(rels)

	res := &BaselineResult{Root: root, Kind: kind}
	total := len(rels)
	for i, rel := range rels {
		if progress != nil {
			progress(i+1, total, rel)
		}
		if incompleteTargets[rel] {
			res.IncompleteList = append(res.IncompleteList, rel)
			continue
		}
		e, err := EntryFromFile(root, rel, LocalSourceID(rel), false)
		if err != nil {
			// 读不出的文件不登记，交给核验/修复暴露，保持保守。
			res.IncompleteList = append(res.IncompleteList, rel)
			continue
		}
		l.Upsert(e)
	}
	markerRels := make([]string, 0, len(incompleteMarkers))
	for rel := range incompleteMarkers {
		markerRels = append(markerRels, rel)
	}
	sort.Strings(markerRels)
	res.IncompleteList = append(res.IncompleteList, markerRels...)
	sort.Strings(res.IncompleteList)

	if err := l.Save(); err != nil {
		return nil, nil, err
	}
	res.EntryCount = l.Count()
	return l, res, nil
}

// AdoptUnregistered 把核验报告中的未登记媒体（或指定 rel 子集）纳入既有账本。
// 新条目一律 local: 身份、origin_verified=false；本函数不改任何媒体文件。
// 返回新纳入条目数；文件在核验后被删除/改动导致无法摘要的项会被跳过。
func AdoptUnregistered(root string, report *VerifyReport, selected []string) (int, error) {
	if report == nil {
		return 0, nil
	}
	l, err := LoadLedger(root)
	if err != nil {
		return 0, err
	}

	want := map[string]bool{}
	for _, rel := range selected {
		want[rel] = true
	}
	added := 0
	for _, item := range report.Section(VerifyUnregistered) {
		if len(want) > 0 && !want[item.RelPath] {
			continue
		}
		// 已有条目（可能刚好是另一个身份指向同路径）则不重复纳入。
		if _, exists := l.FindByRelPath(item.RelPath); exists {
			continue
		}
		e, err := EntryFromFile(root, item.RelPath, LocalSourceID(item.RelPath), false)
		if err != nil {
			continue
		}
		l.Upsert(e)
		added++
	}
	if added == 0 {
		return 0, nil
	}
	return added, l.Save()
}
