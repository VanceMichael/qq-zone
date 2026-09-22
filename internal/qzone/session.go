// 登录会话持久化管理，支持多账号 Session 的保存、加载与切换

package qzone

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	// SessionPath 定义了多账号会话信息的持久化文件路径
	SessionPath = "storage/sessions.json"

	// ErrSessionStorage 表示会话文件本身不可读或内容损坏。
	// 命中该错误时必须停止账号选择和新会话写入，绝不允许把它当成空账号库覆盖原文件。
	ErrSessionStorage = errors.New("本地会话存储异常")
)

// Session 记录了单个账号登录状态的核心凭证与信息
type Session struct {
	QQ        string    `json:"qq"`                   // 账号的唯一标识
	Nickname  string    `json:"nickname"`             // 账号的展示昵称
	GTK       string    `json:"g_tk"`                 // 根据 p_skey 算出的防跨站 CSRF 凭证
	Cookie    string    `json:"cookie"`               // QQ 空间接口 Cookie
	QunCookie string    `json:"qun_cookie,omitempty"` // 群管理页 Cookie，用来列出加入的群
	LastUsed  time.Time `json:"last_used"`            // 最后一次使用的时间
}

// SessionStorageError 携带会话存储故障的具体环节，便于上层向用户展示可观察的错误。
type SessionStorageError struct {
	Op   string // 正在执行的操作，如 读取/解析/写入
	Path string // 出问题的文件路径
	Err  error  // 底层错误
}

func (e *SessionStorageError) Error() string {
	if e == nil {
		return ErrSessionStorage.Error()
	}
	return fmt.Sprintf("%s: %s会话文件 %s 失败: %v", ErrSessionStorage, e.Op, e.Path, e.Err)
}

func (e *SessionStorageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Is 让 errors.Is(err, ErrSessionStorage) 成立，同时 Unwrap 保留底层原因。
func (e *SessionStorageError) Is(target error) bool {
	return target == ErrSessionStorage
}

// IsSessionStorageError 判断错误是否源自会话存储层（文件不可读、内容损坏、写入失败等）。
func IsSessionStorageError(err error) bool {
	return errors.Is(err, ErrSessionStorage)
}

// LoadSessions 从本地 sessions.json 读取当前存储的所有历史账号信息。
// 文件尚不存在（首次使用）时返回空 map 与 nil；文件不可读或内容损坏时返回
// ErrSessionStorage，调用方必须停止后续选择/写入，不能把它当成空账号库。
func LoadSessions() (map[string]*Session, error) {
	sessions := make(map[string]*Session)

	data, err := os.ReadFile(SessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return sessions, nil
		}
		return nil, &SessionStorageError{Op: "读取", Path: SessionPath, Err: err}
	}

	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, &SessionStorageError{Op: "解析", Path: SessionPath, Err: err}
	}

	// 存在 null 条目或缺少账号标识说明文件已损坏，直接报错而不是带着脏数据继续，
	// 否则后续渲染账号列表会 panic，或在保存时悄悄丢掉这些条目。
	for qq, s := range sessions {
		if s == nil {
			return nil, &SessionStorageError{Op: "解析", Path: SessionPath,
				Err: fmt.Errorf("账号 %s 的记录为空", qq)}
		}
		if strings.TrimSpace(s.QQ) == "" {
			s.QQ = qq // 历史文件可能只写了 map key，容错补上而不是判废
		}
	}

	return sessions, nil
}

// SaveSession 将当前最新提取到的登录凭证写入本地存储文件。
// 会合并已有会话（保留其他账号的全部内容），并刷新本次写入账号的 LastUsed。
// 当现有会话文件不可读或损坏时直接返回错误，绝不用空账号库覆盖原文件；
// 写入采用临时文件 + 原子替换，保存失败时旧文件原样保留。
func SaveSession(s *Session) error {
	if s == nil || strings.TrimSpace(s.QQ) == "" {
		return errors.New("无法保存会话：缺少账号标识")
	}

	sessions, err := LoadSessions()
	if err != nil {
		return err
	}

	s.LastUsed = time.Now()
	sessions[s.QQ] = s

	return saveSessionsToFile(sessions)
}

// TouchSession 仅把指定账号的最近使用时间刷新为当前时刻。
// 该账号的群授权（QunCookie）以及其他账号的全部内容都原样保留；
// 只有原子写入成功后刷新才会生效，写入失败时旧文件保持不变并返回错误。
func TouchSession(qq string) error {
	qq = strings.TrimSpace(qq)
	if qq == "" {
		return errors.New("无法刷新会话：缺少账号标识")
	}

	sessions, err := LoadSessions()
	if err != nil {
		return err
	}

	s, ok := sessions[qq]
	if !ok || s == nil {
		// 账号已被主动清理时无需刷新，不算存储故障。
		return nil
	}
	s.LastUsed = time.Now()

	return saveSessionsToFile(sessions)
}

// GetLastSession 从本地存储中提取出最新使用（或最后更新）的活跃会话
func GetLastSession() (*Session, error) {
	sessions, err := LoadSessions()
	if err != nil || len(sessions) == 0 {
		return nil, err
	}

	var list []*Session
	for _, s := range sessions {
		list = append(list, s)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].LastUsed.After(list[j].LastUsed)
	})

	return list[0], nil
}

// RemoveSession 根据提供的 QQ 号，从本地持久化存储中移除指定的登录状态
func RemoveSession(qq string) error {
	sessions, err := LoadSessions()
	if err != nil {
		return err
	}

	if _, ok := sessions[qq]; ok {
		delete(sessions, qq)
		return saveSessionsToFile(sessions)
	}

	return nil
}

// ClearSession 清理当前所有会话 (保留方法名以兼容旧逻辑，实际逻辑为清理最近一次)
func ClearSession() error {
	s, err := GetLastSession()
	if err != nil || s == nil {
		return err
	}
	return RemoveSession(s.QQ)
}

// SetActiveSession 标记指定的 QQ 号为当前激活状态(最新使用)
func SetActiveSession(qq string) error {
	return TouchSession(qq)
}

// HasSession 检查本地是否存在任何已保存的历史登录凭证。
// 存储本身故障时返回 false 仅代表“没有可用凭证”，错误细节请用 LoadSessions 获取。
func HasSession() bool {
	sessions, err := LoadSessions()
	if err != nil {
		return false
	}
	return len(sessions) > 0
}

// saveSessionsToFile 将多账号状态全量、原子地写入本地文件：
// 先写同目录临时文件并落盘，再 rename 替换，任何一步失败都不会破坏旧文件。
func saveSessionsToFile(sessions map[string]*Session) error {
	dir := filepath.Dir(SessionPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return &SessionStorageError{Op: "创建目录", Path: SessionPath, Err: err}
	}

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return &SessionStorageError{Op: "序列化", Path: SessionPath, Err: err}
	}

	perm := os.FileMode(0o644)
	if fi, statErr := os.Stat(SessionPath); statErr == nil {
		perm = fi.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, ".sessions-*.tmp")
	if err != nil {
		return &SessionStorageError{Op: "写入临时文件", Path: SessionPath, Err: err}
	}
	tmpName := tmp.Name()
	// 任何失败分支都清理临时文件，但绝不动旧的 sessions.json。
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return &SessionStorageError{Op: "写入临时文件", Path: SessionPath, Err: err}
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return &SessionStorageError{Op: "写入临时文件", Path: SessionPath, Err: err}
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return &SessionStorageError{Op: "写入临时文件", Path: SessionPath, Err: err}
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		cleanup()
		return &SessionStorageError{Op: "写入临时文件", Path: SessionPath, Err: err}
	}

	if err := os.Rename(tmpName, SessionPath); err != nil {
		cleanup()
		return &SessionStorageError{Op: "替换", Path: SessionPath, Err: err}
	}
	return nil
}

// qunCookieForQQ 取出该号上次授权过的群管理页 Cookie；没有则空。空间重新扫码时用来接着用。
func qunCookieForQQ(qq string) string {
	if qq == "" {
		return ""
	}
	sessions, err := LoadSessions()
	if err != nil || sessions == nil {
		return ""
	}
	if s := sessions[qq]; s != nil {
		return s.QunCookie
	}
	return ""
}
