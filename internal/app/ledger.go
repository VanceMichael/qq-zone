// 媒体完整性账本：四类备份（个人相册/群相册/说说/留言板）各自的持久媒体登记。
// 仅登记真正完成并关闭的文件，条目含稳定来源身份、实际相对路径、字节数与 SHA-256。
// 写入一律走「同目录临时文件写完整 → Sync → 原子 rename」，崩溃或取消后上一份账本仍可解析。

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/qinjintian/qq-zone/internal/pkg/util"
)

const (
	// ledgerVersion 是 ledger.json 的结构版本。
	ledgerVersion = 1
	// ledgerHashAlg 是账本使用的内容摘要算法。
	ledgerHashAlg = "sha256"
	// integrityDirName 是各备份域根下存放账本/修复计划/隔离区的隐藏目录。
	integrityDirName = ".integrity"
	// ledgerFileName / ledgerBackupFileName 分别是当前账本与最近完好备份。
	ledgerFileName       = "ledger.json"
	ledgerBackupFileName = "ledger.bak"
)

// LedgerKind 标识账本属于哪一类备份。
type LedgerKind string

const (
	LedgerKindAlbum      LedgerKind = "album"       // 个人相册（含好友相册），目录 album/
	LedgerKindGroupAlbum LedgerKind = "group_album" // 群相册，目录 qun/<群号>/
	LedgerKindShuoshuo   LedgerKind = "shuoshuo"    // 说说
	LedgerKindBoard      LedgerKind = "board"       // 留言板
)

// 稳定来源身份前缀，格式见 FR-1，任何登记/修复代码都应通过下面的构造器生成。
const (
	sourcePrefixAlbum = "album:"
	sourcePrefixGroup = "group:"
	sourcePrefixShuo  = "shuoshuo:"
	sourcePrefixBoard = "board:"
	sourcePrefixLocal = "local:"
)

// 媒体条目类型。
const (
	MediaKindImage = "image"
	MediaKindVideo = "video"
	MediaKindVoice = "voice"
	MediaKindOther = "other"
)

// LedgerEntry 是单个完成态媒体文件的完整性登记。
type LedgerEntry struct {
	SourceID       string    `json:"source_id"`       // 稳定来源身份（FR-1 格式）
	MediaKind      string    `json:"media_kind"`      // image / video / voice / other
	RelPath        string    `json:"rel_path"`        // 相对域根的 POSIX 风格路径
	Size           int64     `json:"size"`            // 字节数
	SHA256         string    `json:"sha256"`          // 内容摘要（小写十六进制）
	OriginVerified bool      `json:"origin_verified"` // true=字节确实来自成功的在线下载；false=本地基线，未与空间原件核对
	CreatedAt      time.Time `json:"created_at"`      // 该身份首次登记时间
	UpdatedAt      time.Time `json:"updated_at"`      // 最近一次内容更新时间
}

// Ledger 是一个备份域根的完整账本。
type Ledger struct {
	Version   int           `json:"version"`
	Kind      LedgerKind    `json:"kind"`
	OwnerUin  string        `json:"owner_uin"`          // 域根归属 QQ：相册/说说/留言板为目标 QQ，群相册为登录 QQ
	GroupID   string        `json:"group_id,omitempty"` // 仅群相册
	HashAlg   string        `json:"hash_alg"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	Entries   []LedgerEntry `json:"entries"`

	root      string         `json:"-"` // 域根绝对/相对路径，不序列化
	bySource  map[string]int `json:"-"` // source_id → Entries 下标
	byRelPath map[string]int `json:"-"` // rel_path → Entries 下标
}

// LedgerDomain 描述本机发现的一个备份域根，供菜单选择与核验入口使用。
type LedgerDomain struct {
	Kind       LedgerKind
	Root       string // 域根路径
	OwnerUin   string
	GroupID    string
	HasLedger  bool // ledger.json 是否存在
	EntryCount int  // 可解析账本的条目数
	Corrupt    bool // 账本存在但 json/bak 都无法解析
}

// AlbumLedgerRoot 返回个人相册域根 storage/qzone/<QQ>/album。
func AlbumLedgerRoot(targetUin string) string {
	return filepath.Join("storage", "qzone", targetUin, "album")
}

// GroupLedgerRoot 返回群相册域根 storage/qzone/<登录QQ>/qun/<群号>。
func GroupLedgerRoot(operatorUin, groupID string) string {
	return filepath.Join("storage", "qzone", operatorUin, "qun", sanitizePath(groupID))
}

// ShuoshuoLedgerRoot 返回说说域根，复用 moodRoot。
func ShuoshuoLedgerRoot(targetUin string) string {
	return moodRoot(targetUin)
}

// BoardLedgerRoot 返回留言板域根，复用 boardRoot。
func BoardLedgerRoot(targetUin string) string {
	return boardRoot(targetUin)
}

// LedgerPath 返回域根内账本文件路径。
func LedgerPath(root string) string {
	return filepath.Join(root, integrityDirName, ledgerFileName)
}

// AlbumSourceID 构造个人相册媒体的稳定身份：album:<目标QQ>:<相册ID>:<sloc>。
func AlbumSourceID(targetUin, albumID, sloc string) string {
	return sourcePrefixAlbum + strings.Join([]string{targetUin, albumID, sloc}, ":")
}

// GroupSourceID 构造群相册媒体的稳定身份：group:<登录QQ>:<群号>:<相册ID>:<sloc>。
func GroupSourceID(operatorUin, groupID, albumID, sloc string) string {
	return sourcePrefixGroup + strings.Join([]string{operatorUin, groupID, albumID, sloc}, ":")
}

// MoodMediaSourceID 构造说说/留言板媒体的稳定身份。
// prefix 为 shuoshuo: 或 board:；媒体 ID 为空时退化为 url:<MD5(URL)>，与现有文件名派生口径一致。
func MoodMediaSourceID(prefix, targetUin, tid, mediaID, mediaURL string) string {
	if strings.TrimSpace(mediaID) != "" {
		return prefix + strings.Join([]string{targetUin, tid, mediaID}, ":")
	}
	return prefix + strings.Join([]string{targetUin, tid, "url", util.MD5(mediaURL)}, ":")
}

// ShuoshuoSourceID / BoardSourceID 是 MoodMediaSourceID 的类型化包装。
func ShuoshuoSourceID(targetUin, tid, mediaID, mediaURL string) string {
	return MoodMediaSourceID(sourcePrefixShuo, targetUin, tid, mediaID, mediaURL)
}

// BoardSourceID 构造留言板媒体身份。
func BoardSourceID(targetUin, tid, mediaID, mediaURL string) string {
	return MoodMediaSourceID(sourcePrefixBoard, targetUin, tid, mediaID, mediaURL)
}

// LocalSourceID 构造离线基线身份：local:<rel_path>，始终代表来源未验证条目。
func LocalSourceID(relPath string) string {
	return sourcePrefixLocal + relPath
}

// IsLocalSource 判断身份是否为本地基线身份。
func IsLocalSource(sourceID string) bool {
	return strings.HasPrefix(sourceID, sourcePrefixLocal)
}

// MediaKindFromExt 按文件扩展名推断媒体类型。
func MediaKindFromExt(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic", ".heif", ".bmp":
		return MediaKindImage
	case ".mp4", ".mov", ".m4v", ".avi", ".mkv":
		return MediaKindVideo
	case ".mp3", ".m4a", ".aac", ".amr":
		return MediaKindVoice
	default:
		return MediaKindOther
	}
}

// NewLedger 创建一本尚未落盘的空账本。
func NewLedger(kind LedgerKind, ownerUin, groupID, root string) *Ledger {
	now := time.Now()
	l := &Ledger{
		Version:   ledgerVersion,
		Kind:      kind,
		OwnerUin:  ownerUin,
		GroupID:   groupID,
		HashAlg:   ledgerHashAlg,
		CreatedAt: now,
		UpdatedAt: now,
		Entries:   nil,
		root:      root,
	}
	l.rebuildIndex()
	return l
}

// Root 返回账本域根路径。
func (l *Ledger) Root() string { return l.root }

// Count 返回条目数。
func (l *Ledger) Count() int {
	if l == nil {
		return 0
	}
	return len(l.Entries)
}

// rebuildIndex 根据当前 Entries 重建 source/path 下标。
func (l *Ledger) rebuildIndex() {
	l.bySource = make(map[string]int, len(l.Entries))
	l.byRelPath = make(map[string]int, len(l.Entries))
	for i, e := range l.Entries {
		l.bySource[e.SourceID] = i
		if _, exists := l.byRelPath[e.RelPath]; !exists {
			l.byRelPath[e.RelPath] = i
		}
	}
}

// FindBySource 按稳定来源身份查找条目。
func (l *Ledger) FindBySource(sourceID string) (LedgerEntry, bool) {
	if l == nil {
		return LedgerEntry{}, false
	}
	i, ok := l.bySource[sourceID]
	if !ok {
		return LedgerEntry{}, false
	}
	return l.Entries[i], true
}

// FindByRelPath 按相对路径查找条目。
func (l *Ledger) FindByRelPath(relPath string) (LedgerEntry, bool) {
	if l == nil {
		return LedgerEntry{}, false
	}
	i, ok := l.byRelPath[relPath]
	if !ok {
		return LedgerEntry{}, false
	}
	return l.Entries[i], true
}

// Upsert 按 source_id 插入或更新一条；同一 source 保留首次 CreatedAt。
// 已验证（在线下载）条目落账时，同 rel_path 上的 local: 基线条目会被替换，
// 因为该路径的字节已经被空间来源身份正式接管。
func (l *Ledger) Upsert(e LedgerEntry) {
	now := time.Now()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	e.UpdatedAt = now

	// 一个相对路径只能对应一个身份：已验证（在线下载/修复）条目接管同路径上的旧条目，
	// 无论旧条目是 local 基线还是历史空间身份（文件名由 sloc 稳定派生，正常情况下身份相同）。
	if e.OriginVerified {
		if i, ok := l.byRelPath[e.RelPath]; ok && l.Entries[i].SourceID != e.SourceID {
			l.removeAt(i)
		}
	}

	if i, ok := l.bySource[e.SourceID]; ok {
		old := l.Entries[i]
		e.CreatedAt = old.CreatedAt
		l.Entries[i] = e
		// 同一来源改了 rel（如 Content-Type 改扩展名）时，必须摘掉旧 rel 键，避免陈旧命中。
		if old.RelPath != e.RelPath {
			delete(l.byRelPath, old.RelPath)
		}
		l.byRelPath[e.RelPath] = i
		return
	}

	l.Entries = append(l.Entries, e)
	idx := len(l.Entries) - 1
	l.bySource[e.SourceID] = idx
	if _, exists := l.byRelPath[e.RelPath]; !exists {
		l.byRelPath[e.RelPath] = idx
	}
}

// removeAt 删除下标处条目并重建下标（条目规模下足够简单可靠）。
func (l *Ledger) removeAt(i int) {
	if i < 0 || i >= len(l.Entries) {
		return
	}
	l.Entries = append(l.Entries[:i], l.Entries[i+1:]...)
	l.rebuildIndex()
}

// RemoveByRelPathPrefix 删除 rel_path 以 prefix 开头的全部条目。
// 全量（非增量）相册备份整相册重下后，用它以本次成果整体替换该相册旧登记。
// 返回被删除的条目数。
func (l *Ledger) RemoveByRelPathPrefix(prefix string) int {
	if l == nil || prefix == "" {
		return 0
	}
	kept := l.Entries[:0]
	removed := 0
	for _, e := range l.Entries {
		if strings.HasPrefix(e.RelPath, prefix) {
			removed++
			continue
		}
		kept = append(kept, e)
	}
	l.Entries = kept
	l.rebuildIndex()
	return removed
}

// Remove 按 source_id 删除条目，返回是否删过。
func (l *Ledger) Remove(sourceID string) bool {
	if l == nil {
		return false
	}
	i, ok := l.bySource[sourceID]
	if !ok {
		return false
	}
	l.removeAt(i)
	return true
}

// LedgerSkipDecision 是「本地文件能否依据账本直接跳过下载」的纯本地判定。
type LedgerSkipDecision int

const (
	// LedgerSkipNoEntry：账本里没有该来源身份，调用方沿用旧的（HEAD/非空）跳过逻辑。
	LedgerSkipNoEntry LedgerSkipDecision = iota
	// LedgerSkipHealthy：条目登记的路径上现存文件大小与摘要均一致，可离线跳过，不得再发网络请求。
	LedgerSkipHealthy
	// LedgerSkipMismatch：有条目但文件缺失/大小或摘要不一致，必须进入下载链路重建，禁止跳过。
	LedgerSkipMismatch
)

// SkipDecision 按来源身份查账本并核对条目登记路径上的现存文件，全程不访问网络。
// 这是「凡是根据账本声称健康或跳过的媒体，其路径和内容必须与持久记录一致」的统一闸门，
// 个人相册、群相册、说说、留言板四条链路共用。
func (l *Ledger) SkipDecision(sourceID string) LedgerSkipDecision {
	if l == nil {
		return LedgerSkipNoEntry
	}
	e, ok := l.FindBySource(sourceID)
	if !ok {
		return LedgerSkipNoEntry
	}
	fi, err := os.Stat(filepath.Join(l.root, filepath.FromSlash(e.RelPath)))
	if err != nil || !fi.Mode().IsRegular() || fi.Size() <= 0 {
		return LedgerSkipMismatch
	}
	if fi.Size() != e.Size {
		return LedgerSkipMismatch
	}
	sum, err := util.FileSHA256(filepath.Join(l.root, filepath.FromSlash(e.RelPath)))
	if err != nil || sum != e.SHA256 {
		return LedgerSkipMismatch
	}
	return LedgerSkipHealthy
}

// EntryFromFile 对一个已完成的本地文件计算登记条目（字节数 + SHA-256 一次扫描得到）。
// relPath 必须是相对域根的 POSIX 风格路径。
func EntryFromFile(root, relPath, sourceID string, originVerified bool) (LedgerEntry, error) {
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	size, sum, err := util.FileDigest(abs)
	if err != nil {
		return LedgerEntry{}, err
	}
	now := time.Now()
	return LedgerEntry{
		SourceID:       sourceID,
		MediaKind:      MediaKindFromExt(relPath),
		RelPath:        relPath,
		Size:           size,
		SHA256:         sum,
		OriginVerified: originVerified,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Save 以原子替换方式写出 ledger.json，并在替换成功后刷新 ledger.bak。
// 调用方必须先把整次备份/修复的新状态在内存中合并完整，再调用本函数。
func (l *Ledger) Save() error {
	if l == nil {
		return errors.New("nil ledger")
	}
	l.Version = ledgerVersion
	if l.HashAlg == "" {
		l.HashAlg = ledgerHashAlg
	}
	sort.SliceStable(l.Entries, func(i, j int) bool {
		if l.Entries[i].RelPath == l.Entries[j].RelPath {
			return l.Entries[i].SourceID < l.Entries[j].SourceID
		}
		return l.Entries[i].RelPath < l.Entries[j].RelPath
	})
	// 原地重排后必须重建下标，否则 bySource/byRelPath 仍指向旧位置，
	// 多相册分边界提交后会让 SkipDecision/Upsert 命中错误条目（审查 F1）。
	l.rebuildIndex()
	l.UpdatedAt = time.Now()
	if l.CreatedAt.IsZero() {
		l.CreatedAt = l.UpdatedAt
	}

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Join(l.root, integrityDirName)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	current := filepath.Join(dir, ledgerFileName)
	if err := atomicWriteFile(current, data); err != nil {
		return err
	}
	// ledger.bak 只做锦上添花的最近完好副本：写失败不影响本次提交（current 已完整切换）。
	backup := filepath.Join(dir, ledgerBackupFileName)
	if err := atomicWriteFile(backup, data); err != nil {
		return nil //nolint:nilerr // 见注释
	}
	return nil
}

// ErrLedgerMissing 表示域根下没有账本文件。
var ErrLedgerMissing = errors.New("ledger file not found")

// ErrLedgerCorrupt 表示 ledger.json 与 ledger.bak 都无法解析，调用方必须中止而不是静默重建。
var ErrLedgerCorrupt = errors.New("ledger is corrupt: ledger.json and ledger.bak both unparseable")

// LoadLedger 读取域根账本；ledger.json 损坏时自动回退 ledger.bak。
// 账本不存在返回 ErrLedgerMissing；两份都损坏返回 ErrLedgerCorrupt。
func LoadLedger(root string) (*Ledger, error) {
	current := LedgerPath(root)
	l, err := loadLedgerFile(current)
	if err == nil {
		l.root = root
		return l, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		// 当前账本不存在：若只有 bak（极端情况下 rename 后崩溃），也允许恢复。
		backup := filepath.Join(root, integrityDirName, ledgerBackupFileName)
		l, bakErr := loadLedgerFile(backup)
		if bakErr != nil {
			return nil, ErrLedgerMissing
		}
		l.root = root
		return l, nil
	}

	// ledger.json 存在但解析失败：回退最近完好副本。
	backup := filepath.Join(root, integrityDirName, ledgerBackupFileName)
	l, bakErr := loadLedgerFile(backup)
	if bakErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrLedgerCorrupt, err)
	}
	l.root = root
	return l, nil
}

func loadLedgerFile(path string) (*Ledger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var l Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, err
	}
	if len(l.Entries) == 0 {
		l.Entries = nil
	}
	l.rebuildIndex()
	return &l, nil
}

// atomicWriteFile 在同目录写临时文件、Sync 后原子 rename 覆盖目标，
// 保证崩溃时目标要么是旧完整内容、要么是新完整内容，绝不出现半截 JSON。
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()

	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	closed = true
	return os.Rename(tmp, path)
}

// DiscoverLedgerRoots 扫描 storage/qzone 下全部四类备份域根。
func DiscoverLedgerRoots() []LedgerDomain {
	return discoverLedgerRoots("storage")
}

func discoverLedgerRoots(base string) []LedgerDomain {
	var domains []LedgerDomain

	// 个人相册：storage/qzone/<QQ>/album
	albumRoots, _ := filepath.Glob(filepath.Join(base, "qzone", "*", "album"))
	for _, root := range albumRoots {
		if !util.IsDir(root) {
			continue
		}
		ownerUin := filepath.Base(filepath.Dir(root))
		domains = append(domains, describeDomain(LedgerKindAlbum, root, ownerUin, ""))
	}

	// 群相册：storage/qzone/<登录QQ>/qun/<群号>
	groupRoots, _ := filepath.Glob(filepath.Join(base, "qzone", "*", "qun", "*"))
	for _, root := range groupRoots {
		if !util.IsDir(root) {
			continue
		}
		groupID := filepath.Base(root)
		ownerUin := filepath.Base(filepath.Dir(filepath.Dir(root)))
		domains = append(domains, describeDomain(LedgerKindGroupAlbum, root, ownerUin, groupID))
	}

	// 说说
	moodRoots, _ := filepath.Glob(filepath.Join(base, "qzone", "*", moodDirName))
	for _, root := range moodRoots {
		if !util.IsDir(root) {
			continue
		}
		ownerUin := filepath.Base(filepath.Dir(root))
		domains = append(domains, describeDomain(LedgerKindShuoshuo, root, ownerUin, ""))
	}

	// 留言板
	boardRoots, _ := filepath.Glob(filepath.Join(base, "qzone", "*", boardDirName))
	for _, root := range boardRoots {
		if !util.IsDir(root) {
			continue
		}
		ownerUin := filepath.Base(filepath.Dir(root))
		domains = append(domains, describeDomain(LedgerKindBoard, root, ownerUin, ""))
	}

	sort.SliceStable(domains, func(i, j int) bool {
		if domains[i].Kind != domains[j].Kind {
			return domains[i].Kind < domains[j].Kind
		}
		if domains[i].OwnerUin != domains[j].OwnerUin {
			return domains[i].OwnerUin < domains[j].OwnerUin
		}
		return domains[i].GroupID < domains[j].GroupID
	})
	return domains
}

// describeDomain 读取单个域根的账本状态（有无/条目数/损坏），读失败只做状态标记不抛错。
func describeDomain(kind LedgerKind, root, ownerUin, groupID string) LedgerDomain {
	d := LedgerDomain{Kind: kind, Root: root, OwnerUin: ownerUin, GroupID: groupID}
	if _, err := os.Stat(LedgerPath(root)); err == nil {
		d.HasLedger = true
		if l, loadErr := LoadLedger(root); loadErr == nil {
			d.EntryCount = l.Count()
		} else if errors.Is(loadErr, ErrLedgerCorrupt) {
			d.Corrupt = true
		}
	}
	return d
}
