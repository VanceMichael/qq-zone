// 定点修复：由离线核验结果生成持久修复计划，隔离确认损坏/未完成文件，
// 再按原备份类型的既有取源与重试链路（Spider / MoodBackup / BoardBackup）重建。
// 健康文件不重写；计划原子持久化，无凭证、断网或取消后下次可继续。

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/qinjintian/qq-zone/internal/pkg/util"
	"go.uber.org/zap"
)

const (
	repairFileName = "repair.json"
	repairVersion  = 1

	// RepairReason* 与核验分类对应（未登记不入计划）。
	RepairReasonMissing    = "missing"
	RepairReasonChanged    = "changed"
	RepairReasonIncomplete = "incomplete"
)

// RepairItem 是一个等待定点重建的媒体。
type RepairItem struct {
	SourceID       string    `json:"source_id"`                 // 稳定来源身份；孤立半成品可能为 local:<rel>
	RelPath        string    `json:"rel_path"`                  // 最终目标媒体相对域根路径
	MarkerRelPath  string    `json:"marker_rel_path,omitempty"` // 已知的未完成标记（.part/.resume.json），可空
	Reason         string    `json:"reason"`                    // missing / changed / incomplete
	MediaKind      string    `json:"media_kind,omitempty"`
	AutoRepairable bool      `json:"auto_repairable"` // 能否用现有在线链路自动修复
	AlbumName      string    `json:"album_name,omitempty"`
	AlbumID        string    `json:"album_id,omitempty"`
	TID            string    `json:"tid,omitempty"`
	MediaID        string    `json:"media_id,omitempty"`
	MediaURL       string    `json:"media_url,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// RepairPlan 是跨运行续作的修复计划，落盘在域根 .integrity/repair.json。
type RepairPlan struct {
	Version   int          `json:"version"`
	Kind      LedgerKind   `json:"kind"`
	OwnerUin  string       `json:"owner_uin"`
	GroupID   string       `json:"group_id,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	Items     []RepairItem `json:"items"`

	root string `json:"-"`
}

// RepairPlanPath 返回计划文件路径。
func RepairPlanPath(root string) string {
	return filepath.Join(root, integrityDirName, repairFileName)
}

// ErrRepairPlanMissing 表示域根下没有修复计划。
var ErrRepairPlanMissing = errors.New("repair plan not found")

// LoadRepairPlan 读取修复计划；文件不存在返回 ErrRepairPlanMissing。
func LoadRepairPlan(root string) (*RepairPlan, error) {
	data, err := os.ReadFile(RepairPlanPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrRepairPlanMissing
		}
		return nil, err
	}
	var p RepairPlan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	p.root = root
	return &p, nil
}

// HasPendingRepair 判断域根是否存在非空修复计划。
func HasPendingRepair(root string) bool {
	p, err := LoadRepairPlan(root)
	return err == nil && len(p.Items) > 0
}

// Save 原子写出计划；条目为空时删除计划文件（全部修复完成）。
func (p *RepairPlan) Save() error {
	if len(p.Items) == 0 {
		if err := os.Remove(RepairPlanPath(p.root)); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	p.Version = repairVersion
	p.UpdatedAt = time.Now()
	sort.SliceStable(p.Items, func(i, j int) bool {
		if p.Items[i].Reason != p.Items[j].Reason {
			return p.Items[i].Reason < p.Items[j].Reason
		}
		return p.Items[i].RelPath < p.Items[j].RelPath
	})
	if err := os.MkdirAll(filepath.Join(p.root, integrityDirName), os.ModePerm); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(RepairPlanPath(p.root), data)
}

// Remove 丢弃整份计划（需用户显式确认后由 CLI 调用）。
func (p *RepairPlan) Remove() error {
	if err := os.Remove(RepairPlanPath(p.root)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// NewRepairPlan 从核验报告生成计划：缺失/变更/未完成入计划，未登记不入。
func NewRepairPlan(domain LedgerDomain, report *VerifyReport) *RepairPlan {
	now := time.Now()
	p := &RepairPlan{
		Version:   repairVersion,
		Kind:      domain.Kind,
		OwnerUin:  domain.OwnerUin,
		GroupID:   domain.GroupID,
		CreatedAt: now,
		root:      domain.Root,
	}

	seen := map[string]bool{}
	add := func(vi VerifyItem) {
		it := repairItemFromVerify(domain, vi)
		if seen[it.RelPath] {
			return
		}
		seen[it.RelPath] = true
		it.CreatedAt = now
		p.Items = append(p.Items, it)
	}
	for _, s := range []string{VerifyMissing, VerifyChanged, VerifyIncomplete} {
		for _, vi := range report.Section(s) {
			add(vi)
		}
	}
	return p
}

// repairItemFromVerify 把单条核验明细转成计划项并补齐取源提示。
func repairItemFromVerify(domain LedgerDomain, vi VerifyItem) RepairItem {
	target, marker := splitMarker(vi)
	it := RepairItem{
		SourceID:      vi.SourceID,
		RelPath:       target,
		MarkerRelPath: marker,
		Reason:        verifyReasonToRepair(vi.Status),
		MediaKind:     vi.MediaKind,
	}
	if it.RelPath == "" {
		it.RelPath = vi.RelPath
	}

	switch domain.Kind {
	case LedgerKindAlbum, LedgerKindGroupAlbum:
		it.AlbumName = firstPathSegment(it.RelPath)
		it.AlbumID = albumIDFromSource(vi.SourceID)
		if it.SourceID == "" {
			it.SourceID = LocalSourceID(it.RelPath)
		}
		// 只要能定位到相册（按相册名白名单或 ID），在线重跑就能按文件名补回。
		it.AutoRepairable = it.AlbumName != ""
	case LedgerKindShuoshuo, LedgerKindBoard:
		fillMoodRepairHints(domain, &it)
	}
	return it
}

// splitMarker 从未完成明细中拆出最终目标 rel 与标记 rel。
func splitMarker(vi VerifyItem) (target, marker string) {
	rel := vi.RelPath
	if strings.HasSuffix(rel, hlsPartExt2) {
		return strings.TrimSuffix(rel, hlsPartExt2), rel
	}
	if strings.HasSuffix(rel, resumeSidecarExt) {
		return strings.TrimSuffix(rel, resumeSidecarExt), rel
	}
	// 有条目的未完成：RelPath 是目标，Detail 形如「存在未完成标记: xxx」。
	const prefix = "存在未完成标记: "
	if strings.HasPrefix(vi.Detail, prefix) {
		return rel, strings.TrimPrefix(vi.Detail, prefix)
	}
	return rel, ""
}

func verifyReasonToRepair(status string) string {
	switch status {
	case VerifyMissing:
		return RepairReasonMissing
	case VerifyIncomplete:
		return RepairReasonIncomplete
	default:
		return RepairReasonChanged
	}
}

func firstPathSegment(rel string) string {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	if i := strings.Index(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return ""
}

// albumIDFromSource 从 album:/group: 身份里解析相册 ID；local/其他身份返回空。
func albumIDFromSource(sourceID string) string {
	switch {
	case strings.HasPrefix(sourceID, sourcePrefixGroup):
		// group:<uin>:<gid>:<albumID>:<sloc>
		parts := strings.SplitN(sourceID, ":", 5)
		if len(parts) == 5 {
			return parts[3]
		}
	case strings.HasPrefix(sourceID, sourcePrefixAlbum):
		// album:<uin>:<albumID>:<sloc>
		parts := strings.SplitN(sourceID, ":", 4)
		if len(parts) == 4 {
			return parts[2]
		}
	}
	return ""
}

// fillMoodRepairHints 从 source_id 与本地 backup.json 补齐说说/留言板修复所需的 tid/媒体/URL。
func fillMoodRepairHints(domain LedgerDomain, it *RepairItem) {
	tid, mediaID, mediaURL := parseMoodSourceHint(it.SourceID)
	it.TID, it.MediaID, it.MediaURL = tid, mediaID, mediaURL

	// source 里只有 URL 摘要或完全没有身份（孤立半成品）时，用本地 backup.json 按路径找回。
	var posts []MoodPost
	switch domain.Kind {
	case LedgerKindShuoshuo:
		if f, err := loadMoodBackup(domain.Root); err == nil {
			posts = f.Posts
		}
	case LedgerKindBoard:
		if f, err := loadBoardBackup(domain.Root); err == nil {
			posts = f.Posts
		}
	}
	if len(posts) > 0 {
		if ownerTID, m := findBackupMediaByRel(posts, it.RelPath); m != nil {
			it.TID = ownerTID
			if m.ID != "" {
				it.MediaID = m.ID
			}
			if m.URL != "" {
				it.MediaURL = m.URL
			}
			if it.SourceID == "" {
				prefix := sourcePrefixShuo
				if domain.Kind == LedgerKindBoard {
					prefix = sourcePrefixBoard
				}
				it.SourceID = MoodMediaSourceID(prefix, domain.OwnerUin, ownerTID, m.ID, m.URL)
			}
		}
	}
	if it.SourceID == "" && it.RelPath != "" {
		it.SourceID = LocalSourceID(it.RelPath)
	}
	it.AutoRepairable = it.TID != "" && (it.MediaID != "" || it.MediaURL != "")
}

// parseMoodSourceHint 解析 shuoshuo:/board: 身份里的 tid 与媒体要素。
func parseMoodSourceHint(sourceID string) (tid, mediaID, mediaURL string) {
	if strings.HasPrefix(sourceID, sourcePrefixShuo) || strings.HasPrefix(sourceID, sourcePrefixBoard) {
		rest := strings.SplitN(sourceID, ":", 2)[1]
		parts := strings.Split(rest, ":")
		switch len(parts) {
		case 3:
			return parts[1], parts[2], "" // <uin>:<tid>:<mediaID>
		case 4:
			if parts[2] == "url" {
				return parts[1], "", "" // <uin>:<tid>:url:<md5>
			}
		}
	}
	return "", "", ""
}

// findBackupMediaByRel 在说说/留言备份树里按本地相对路径找回媒体及其所属条目 tid。
func findBackupMediaByRel(posts []MoodPost, rel string) (string, *MoodMedia) {
	var walk func(tid string, cs []MoodComment) *MoodMedia
	walk = func(tid string, cs []MoodComment) *MoodMedia {
		for i := range cs {
			for j := range cs[i].Media {
				if cs[i].Media[j].Path == rel {
					return &cs[i].Media[j]
				}
			}
			if found := walk(tid, cs[i].Replies); found != nil {
				return found
			}
		}
		return nil
	}
	for i := range posts {
		p := &posts[i]
		for j := range p.Media {
			if p.Media[j].Path == rel {
				return p.TID, &p.Media[j]
			}
		}
		if p.Repost != nil {
			for j := range p.Repost.Media {
				if p.Repost.Media[j].Path == rel {
					return p.TID, &p.Repost.Media[j]
				}
			}
		}
		if found := walk(p.TID, p.Comments); found != nil {
			return p.TID, found
		}
	}
	return "", nil
}

// RepairRunner 抽象各备份类型的在线重建链路，生产实现复用 Spider/MoodBackup/BoardBackup。
type RepairRunner interface {
	RunAlbumRepair(ctx context.Context, kind LedgerKind, ownerUin, groupID string, selectors []string) (*DownloadResult, error)
	RunMoodRepair(ctx context.Context, ownerUin string, items []FailedItem) (*DownloadResult, error)
	RunBoardRepair(ctx context.Context, ownerUin string, items []FailedItem) (*DownloadResult, error)
}

// RepairOutcome 是一次修复运行的结果。
type RepairOutcome struct {
	Resolved     []RepairItem // 本次重建成功并重新登记
	Remaining    []RepairItem // 仍失败（计划保留，下次继续）
	Unrepairable []RepairItem // 无法自动修复（如缺少取源线索）
	Result       *DownloadResult
}

// ExecuteRepair 执行修复：隔离 → 原链路重建 → 账本核对 → 计划移除成功项并原子保存。
// 本函数不处理登录；调用方必须在登录成功后调用，否则直接返回错误且计划原样保留。
func ExecuteRepair(ctx context.Context, domain LedgerDomain, plan *RepairPlan, runner RepairRunner, logger *zap.SugaredLogger) (*RepairOutcome, error) {
	if plan == nil || runner == nil {
		return nil, fmt.Errorf("repair plan/runner 不能为空")
	}
	outcome := &RepairOutcome{Result: &DownloadResult{}}

	qRoot := filepath.Join(domain.Root, integrityDirName, "quarantine", time.Now().Format("20060101-150405"))

	var runnable []RepairItem
	for _, it := range plan.Items {
		if !it.AutoRepairable {
			outcome.Unrepairable = append(outcome.Unrepairable, it)
			continue
		}
		// 只隔离计划内路径：目标文件（changed/incomplete）及它的 sidecar/.part 标记。
		quarantineItem(domain.Root, qRoot, it, logger)
		runnable = append(runnable, it)
	}

	if len(runnable) > 0 {
		switch domain.Kind {
		case LedgerKindAlbum, LedgerKindGroupAlbum:
			selectors := albumSelectors(runnable)
			res, err := runner.RunAlbumRepair(ctx, domain.Kind, plan.OwnerUin, plan.GroupID, selectors)
			collectResult(outcome.Result, res)
			if err != nil {
				// 链路级异常（网络/登录）：计划保留不动，下次可继续。
				return outcome, err
			}
			applyFailedErrors(plan, res)
		case LedgerKindShuoshuo:
			res, err := runner.RunMoodRepair(ctx, plan.OwnerUin, moodFailedItems(FailedKindShuoShuo, "说说", plan.OwnerUin, runnable))
			collectResult(outcome.Result, res)
			if err != nil {
				return outcome, err
			}
			applyFailedErrors(plan, res)
		case LedgerKindBoard:
			res, err := runner.RunBoardRepair(ctx, plan.OwnerUin, moodFailedItems(FailedKindBoard, "留言板", plan.OwnerUin, runnable))
			collectResult(outcome.Result, res)
			if err != nil {
				return outcome, err
			}
			applyFailedErrors(plan, res)
		}
	}

	// 以账本 + 磁盘事实裁决每个计划项：rel 路径上的条目与文件大小/摘要一致才算成功。
	// 同时接受「同一来源身份在新 rel 路径上健康」（Content-Type 改扩展名后文件改名）。
	l, _ := LoadLedger(domain.Root)
	remaining := append([]RepairItem(nil), outcome.Unrepairable...)
	for _, it := range runnable {
		if ledgerRelHealthy(l, it.RelPath) || ledgerSourceResolved(l, it.SourceID) {
			outcome.Resolved = append(outcome.Resolved, it)
			continue
		}
		if it.LastError == "" {
			it.LastError = "链路执行后文件仍未达到账本健康状态（可能已不存在于空间或仍在失败列表）"
		}
		it.UpdatedAt = time.Now()
		outcome.Remaining = append(outcome.Remaining, it)
		remaining = append(remaining, it)
	}

	plan.Items = remaining
	if err := plan.Save(); err != nil && logger != nil {
		logger.Warnf("⚠️ 修复计划写回失败: %v", err)
	}
	return outcome, nil
}

// albumSelectors 收集相册修复白名单：相册名 + 相册 ID，保证特殊字符相册名也能命中。
func albumSelectors(items []RepairItem) []string {
	seen := map[string]bool{}
	var out []string
	add := func(v string) {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, it := range items {
		add(it.AlbumName)
		add(it.AlbumID)
	}
	return out
}

// moodFailedItems 把说说/留言板计划项转成现有 RetryFailed 链路需要的 FailedItem。
func moodFailedItems(kind, album, owner string, items []RepairItem) []FailedItem {
	out := make([]FailedItem, 0, len(items))
	for _, it := range items {
		out = append(out, FailedItem{
			Kind:      kind,
			Album:     album,
			Name:      it.RelPath,
			TargetUin: owner,
			MoodTID:   it.TID,
			MediaID:   it.MediaID,
			MediaURL:  it.MediaURL,
			IsVideo:   it.MediaKind == MediaKindVideo,
		})
	}
	return out
}

// quarantineItem 把计划内目标及其未完成标记移动到隔离区；缺失文件无需处理。
func quarantineItem(root, qRoot string, it RepairItem, logger *zap.SugaredLogger) {
	candidates := []string{it.RelPath}
	if it.MarkerRelPath != "" && it.MarkerRelPath != it.RelPath {
		candidates = append(candidates, it.MarkerRelPath)
	}
	// 标记路径可能在核验后变化，目标的两类标准标记也一并尝试。
	candidates = append(candidates, it.RelPath+resumeSidecarExt, it.RelPath+hlsPartExt2)

	for _, rel := range candidates {
		if rel == "" || strings.HasPrefix(rel, "..") {
			continue
		}
		src := filepath.Join(root, filepath.FromSlash(rel))
		fi, err := os.Stat(src)
		if err != nil || fi.IsDir() {
			continue
		}
		dst := filepath.Join(qRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
			if logger != nil {
				logger.Warnf("⚠️ 建立隔离目录失败 %s: %v", rel, err)
			}
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			if logger != nil {
				logger.Warnf("⚠️ 隔离文件失败 %s: %v", rel, err)
			}
			continue
		}
		if logger != nil {
			logger.Infof("🧹 已隔离待修复文件: %s", rel)
		}
	}
}

// ledgerRelHealthy 判断账本中登记在 rel 路径上的条目当前是否与磁盘一致。
func ledgerRelHealthy(l *Ledger, rel string) bool {
	if l == nil {
		return false
	}
	e, ok := l.FindByRelPath(rel)
	if !ok {
		return false
	}
	fi, err := os.Stat(filepath.Join(l.Root(), filepath.FromSlash(rel)))
	if err != nil || !fi.Mode().IsRegular() || fi.Size() != e.Size {
		return false
	}
	sum, err := util.FileSHA256(filepath.Join(l.Root(), filepath.FromSlash(rel)))
	return err == nil && sum == e.SHA256
}

// ledgerSourceResolved 判断来源身份当前是否在（可能改名后的）路径上健康。
// local 基线身份不适用——它本身就等于 rel，裁决交给 ledgerRelHealthy。
func ledgerSourceResolved(l *Ledger, sourceID string) bool {
	if l == nil || IsLocalSource(sourceID) {
		return false
	}
	return l.SkipDecision(sourceID) == LedgerSkipHealthy
}

// collectResult 合并一次链路运行的计数。
func collectResult(dst, src *DownloadResult) {
	if src == nil {
		return
	}
	dst.Total += src.Total
	dst.Success += src.Success
	dst.NewAdded += src.NewAdded
	dst.Skipped += src.Skipped
	dst.Failed += src.Failed
	dst.VideoCount += src.VideoCount
	dst.ImageCount += src.ImageCount
	dst.BytesDone += src.BytesDone
	dst.FailedItems = append(dst.FailedItems, src.FailedItems...)
}

// applyFailedErrors 把链路失败原因写回对应计划项（按 rel 路径或文件名匹配）。
func applyFailedErrors(plan *RepairPlan, res *DownloadResult) {
	if res == nil {
		return
	}
	byRel := map[string]string{}
	byBase := map[string]string{}
	for _, f := range res.FailedItems {
		byRel[f.Name] = f.Error
		byBase[filepath.Base(f.Name)] = f.Error
	}
	for i := range plan.Items {
		it := &plan.Items[i]
		if msg := byRel[it.RelPath]; msg != "" {
			it.LastError = msg
			continue
		}
		if msg := byBase[filepath.Base(it.RelPath)]; msg != "" {
			it.LastError = msg
		}
	}
}
