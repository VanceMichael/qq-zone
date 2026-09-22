// 说说与留言板共用的完整性账本会话：打开账本、来源身份、跳过决策、完成条目暂存与原子提交。
// 两条备份链路对媒体的处理口径必须一致，统一收敛在这里，避免各写一套。

package app

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// mediaLedgerSession 挂载在 MoodBackup / BoardBackup 上，语义与 Spider 的 staged 提交相同。
type mediaLedgerSession struct {
	mu     sync.Mutex
	ledger *Ledger
	root   string // 域根（shuoshuo / liuyanban）
	prefix string // sourcePrefixShuo 或 sourcePrefixBoard
	uin    string // 被备份空间 QQ
	staged []LedgerEntry
}

// init 打开域根账本；不存在则新建内存账本，双损坏时返回错误中止任务。
func (s *mediaLedgerSession) init(kind LedgerKind, prefix, uin, root string) error {
	l, err := LoadLedger(root)
	if errors.Is(err, ErrLedgerMissing) {
		l = NewLedger(kind, uin, "", root)
		err = nil
	}
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.ledger = l
	s.root = root
	s.prefix = prefix
	s.uin = uin
	s.staged = nil
	s.mu.Unlock()
	return nil
}

// sourceID 按条目 tid + 媒体身份生成稳定来源 ID。
func (s *mediaLedgerSession) sourceID(tid string, m *MoodMedia) string {
	if m == nil {
		return ""
	}
	return MoodMediaSourceID(s.prefix, s.uin, tid, m.ID, m.URL)
}

// decisionFor 按完整来源要素做跳过决策（失败项上只有 id/url、内存里没有 MoodMedia 时用）。
func (s *mediaLedgerSession) decisionFor(tid, mediaID, mediaURL string) LedgerSkipDecision {
	if s == nil || s.ledger == nil {
		return LedgerSkipNoEntry
	}
	id := MoodMediaSourceID(s.prefix, s.uin, tid, mediaID, mediaURL)
	return s.ledger.SkipDecision(id)
}

// decision 对一个内存中的媒体做跳过决策。
func (s *mediaLedgerSession) decision(tid string, m *MoodMedia) LedgerSkipDecision {
	if m == nil {
		return LedgerSkipNoEntry
	}
	return s.decisionFor(tid, m.ID, m.URL)
}

// stage 对刚成功下载（非跳过）的媒体计算摘要并暂存；调用时媒体 Path 已是最终相对路径。
func (s *mediaLedgerSession) stage(tid string, m *MoodMedia) {
	if s == nil || s.ledger == nil || m == nil || m.Path == "" {
		return
	}
	rel := filepath.ToSlash(m.Path)
	if rel == "." || strings.HasPrefix(rel, "..") {
		return
	}
	entry, err := EntryFromFile(s.root, rel, s.sourceID(tid, m), true)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.staged = append(s.staged, entry)
	s.mu.Unlock()
}

// commit 在整次备份/重试正常结束时原子提交；clean=false（取消）丢弃暂存、保留旧账本。
func (s *mediaLedgerSession) commit(clean bool, logger *zap.SugaredLogger) {
	if s == nil || s.ledger == nil {
		return
	}
	s.mu.Lock()
	staged := s.staged
	s.staged = nil
	s.mu.Unlock()
	if !clean {
		return
	}
	for _, e := range staged {
		s.ledger.Upsert(e)
	}
	if err := s.ledger.Save(); err != nil && logger != nil {
		logger.Warnf("⚠️ 完整性账本提交失败（不影响已下载文件；这些文件在下次被重新在线下载时会补登记，也可在完整性核验后纳入本地基线）: %v", err)
	}
}
