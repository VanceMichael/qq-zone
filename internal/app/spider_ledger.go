// 相册/群相册爬虫与完整性账本的衔接：打开账本、暂存完成条目、在相册边界原子提交。
// 只有下载链路成功返回的最终文件才会 stage；失败项、跳过项（不改变账本）都不进暂存区。

package app

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
)

// initLedger 在一次下载/重试开始时打开域根账本。
// 账本不存在时新建内存账本（首次提交才落盘）；账本与备份双损坏时中止任务，禁止用空账本覆盖。
func (s *Spider) initLedger(targetUin string) error {
	kind := LedgerKindAlbum
	if s.isGroupMode() {
		kind = LedgerKindGroupAlbum
	}
	root := s.ledgerDomainRoot(targetUin)

	l, err := LoadLedger(root)
	if errors.Is(err, ErrLedgerMissing) {
		groupID := ""
		if s.isGroupMode() {
			groupID = s.groupID
		}
		l = NewLedger(kind, targetUin, groupID, root)
		err = nil
	}
	if err != nil {
		return err
	}

	s.ledgerMu.Lock()
	s.ledger = l
	s.ledgerRoot = root
	s.staged = nil
	s.ledgerMu.Unlock()
	return nil
}

// ledgerDomainRoot 返回当前 Spider 对应的账本域根（个人 album / 群 qun/<gid>）。
func (s *Spider) ledgerDomainRoot(targetUin string) string {
	if s.isGroupMode() {
		return GroupLedgerRoot(targetUin, s.groupID)
	}
	return AlbumLedgerRoot(targetUin)
}

// albumMediaSourceID 生成相册内一张照片/视频的稳定来源身份。
func (s *Spider) albumMediaSourceID(targetUin string, album gjson.Result, sloc string) string {
	albumID := album.Get("id").String()
	if s.isGroupMode() {
		return GroupSourceID(targetUin, s.groupID, albumID, sloc)
	}
	return AlbumSourceID(targetUin, albumID, sloc)
}

// stageCompletedMedia 对一个真正写完并关闭的最终文件计算摘要并放入暂存区。
// 只在下载成功（非账本跳过）后调用；路径逃逸域根或摘要失败时记调试日志且不登记。
func (s *Spider) stageCompletedMedia(sourceID, actualTarget string) {
	if s == nil || s.ledger == nil || sourceID == "" {
		return
	}
	rel, err := filepath.Rel(s.ledgerRoot, actualTarget)
	if err != nil {
		s.logger.Debugf("ledger: 无法计算相对路径 %s: %v", actualTarget, err)
		return
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || strings.HasPrefix(rel, "..") {
		s.logger.Debugf("ledger: 跳过域根之外的文件 %s", actualTarget)
		return
	}
	entry, err := EntryFromFile(s.ledgerRoot, rel, sourceID, true)
	if err != nil {
		s.logger.Debugf("ledger: 登记摘要失败 %s: %v", actualTarget, err)
		return
	}

	s.ledgerMu.Lock()
	s.staged = append(s.staged, entry)
	s.ledgerMu.Unlock()
}

// drainStaged 取出并清空暂存区。
func (s *Spider) drainStaged() []LedgerEntry {
	s.ledgerMu.Lock()
	defer s.ledgerMu.Unlock()
	staged := s.staged
	s.staged = nil
	return staged
}

// commitAlbumLedger 在单个相册完整跑完时把本相册成果合并进账本并原子提交。
// fullReplace=true（非增量）时先整体移除该相册目录前缀的旧条目；
// clean=false（任务被取消/出错未到干净边界）时丢弃本相册暂存，保留上一份账本。
func (s *Spider) commitAlbumLedger(albumPath string, fullReplace, clean bool) {
	if s == nil || s.ledger == nil {
		return
	}
	staged := s.drainStaged()
	if !clean {
		return
	}

	if fullReplace {
		if rel, err := filepath.Rel(s.ledgerRoot, albumPath); err == nil {
			prefix := filepath.ToSlash(rel) + "/"
			if !strings.HasPrefix(prefix, "..") {
				s.ledger.RemoveByRelPathPrefix(prefix)
			}
		}
	}
	for _, e := range staged {
		s.ledger.Upsert(e)
	}
	if err := s.ledger.Save(); err != nil {
		s.logger.Warnf("⚠️ 完整性账本提交失败（不影响已下载文件；这些文件在下次被重新在线下载时会补登记，也可在完整性核验后纳入本地基线）: %v", err)
	}
}

// commitStagedLedger 用于失败重试这类跨相册任务：结束时一次性合并提交；取消时保留旧账本。
func (s *Spider) commitStagedLedger(clean bool) {
	if s == nil || s.ledger == nil {
		return
	}
	staged := s.drainStaged()
	if !clean {
		return
	}
	for _, e := range staged {
		s.ledger.Upsert(e)
	}
	if err := s.ledger.Save(); err != nil {
		s.logger.Warnf("⚠️ 完整性账本提交失败（不影响已下载文件；这些文件在下次被重新在线下载时会补登记，也可在完整性核验后纳入本地基线）: %v", err)
	}
}
