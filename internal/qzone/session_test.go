// 登录会话保存与加载单元测试

package qzone

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func useTempSessionStore(t *testing.T) string {
	t.Helper()
	old := SessionPath
	dir := t.TempDir()
	SessionPath = filepath.Join(dir, "sessions.json")
	t.Cleanup(func() { SessionPath = old })
	return SessionPath
}

func TestQunCookieForQQPreservedAcrossSave(t *testing.T) {
	useTempSessionStore(t)

	if got := qunCookieForQQ("514092640"); got != "" {
		t.Fatalf("empty store = %q", got)
	}
	if err := SaveSession(&Session{
		QQ:        "514092640",
		Nickname:  "nobody",
		Cookie:    "uin=o514092640; p_skey=zone",
		QunCookie: "uin=o514092640; p_skey=qun",
	}); err != nil {
		t.Fatal(err)
	}
	if got := qunCookieForQQ("514092640"); got != "uin=o514092640; p_skey=qun" {
		t.Fatalf("got %q", got)
	}
	if got := qunCookieForQQ("10000"); got != "" {
		t.Fatalf("other qq = %q", got)
	}
}

func TestNewClientFromSessionKeepsQunCookie(t *testing.T) {
	c := NewClientFromSession(&Session{
		QQ:        "514092640",
		Cookie:    "zone",
		QunCookie: "p_skey=qun",
	}, nil, nil)
	if !c.HasQunAuth() || c.QunCookie != "p_skey=qun" {
		t.Fatalf("client = %+v", c)
	}
}

// 损坏的会话文件必须显式报错，任何写入路径都不能把它当成空库覆盖掉。
func TestCorruptSessionStoreNeverOverwritten(t *testing.T) {
	path := useTempSessionStore(t)
	original := []byte("{this-is-not-json")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadSessions(); !IsSessionStorageError(err) {
		t.Fatalf("LoadSessions on corrupt file err = %v, want session storage error", err)
	}
	if _, err := GetLastSession(); !IsSessionStorageError(err) {
		t.Fatalf("GetLastSession err = %v", err)
	}
	if err := SaveSession(&Session{QQ: "1", Cookie: "x"}); !IsSessionStorageError(err) {
		t.Fatalf("SaveSession err = %v, want session storage error", err)
	}
	if err := TouchSession("1"); !IsSessionStorageError(err) {
		t.Fatalf("TouchSession err = %v, want session storage error", err)
	}
	if err := RemoveSession("1"); !IsSessionStorageError(err) {
		t.Fatalf("RemoveSession err = %v, want session storage error", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("corrupt file was overwritten:\nbefore: %s\nafter:  %s", original, got)
	}
}

// null 条目属于损坏文件，而不是空账号库。
func TestNullEntryTreatedAsCorrupt(t *testing.T) {
	path := useTempSessionStore(t)
	if err := os.WriteFile(path, []byte(`{"123456":null}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSessions(); !IsSessionStorageError(err) {
		t.Fatalf("err = %v, want session storage error", err)
	}
}

// 文件不可读（目标路径是目录）时同样按存储故障处理。
func TestUnreadableSessionStore(t *testing.T) {
	old := SessionPath
	dir := t.TempDir()
	SessionPath = dir // 目录无法被 ReadFile 当文件读
	t.Cleanup(func() { SessionPath = old })

	if _, err := LoadSessions(); !IsSessionStorageError(err) {
		t.Fatalf("err = %v, want session storage error", err)
	}
	if err := SaveSession(&Session{QQ: "1"}); !IsSessionStorageError(err) {
		t.Fatalf("SaveSession err = %v, want session storage error", err)
	}
}

// 写入成功后不得残留临时文件；多个账号、群授权必须完整保留。
func TestSaveMergesAccountsAndCleansTempFiles(t *testing.T) {
	path := useTempSessionStore(t)
	dir := filepath.Dir(path)

	if err := SaveSession(&Session{QQ: "111", Nickname: "A", Cookie: "c1", QunCookie: "qun1"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveSession(&Session{QQ: "222", Nickname: "B", Cookie: "c2"}); err != nil {
		t.Fatal(err)
	}

	sessions, err := LoadSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 || sessions["111"].QunCookie != "qun1" || sessions["222"].Nickname != "B" {
		t.Fatalf("merged sessions = %+v", sessions)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".sessions-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

// TouchSession 只刷新指定账号的最近使用时间：群授权保留，其他账号内容一字不动。
func TestTouchSessionOnlyUpdatesLastUsed(t *testing.T) {
	useTempSessionStore(t)

	// 截断到秒：JSON 往返会丢掉单调时钟和亚秒精度。
	oldTime := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	if err := SaveSession(&Session{QQ: "111", Nickname: "A", Cookie: "c1", QunCookie: "qun1"}); err != nil {
		t.Fatal(err)
	}
	// 直接把两个账号的 LastUsed 改成确定的旧值。
	sessions, err := LoadSessions()
	if err != nil {
		t.Fatal(err)
	}
	sessions["111"].LastUsed = oldTime
	sessions["222"] = &Session{QQ: "222", Nickname: "B", Cookie: "c2", QunCookie: "qun2", LastUsed: oldTime}
	if err := saveSessionsToFile(sessions); err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Millisecond)
	if err := TouchSession("111"); err != nil {
		t.Fatal(err)
	}

	got, err := LoadSessions()
	if err != nil {
		t.Fatal(err)
	}
	if !got["111"].LastUsed.After(oldTime) {
		t.Fatalf("selected account LastUsed not refreshed: %v", got["111"].LastUsed)
	}
	if got["111"].QunCookie != "qun1" || got["111"].Cookie != "c1" || got["111"].Nickname != "A" {
		t.Fatalf("selected account content changed: %+v", got["111"])
	}
	other := got["222"]
	if other.LastUsed != oldTime || other.QunCookie != "qun2" || other.Cookie != "c2" || other.Nickname != "B" {
		t.Fatalf("other account content changed: %+v", other)
	}

	// 刷新一个已经不存在的账号不算错误（可能已被用户主动清理）。
	if err := TouchSession("999"); err != nil {
		t.Fatalf("touch missing account: %v", err)
	}
}

// 保存缺少 QQ 的会话必须被拒绝。
func TestSaveSessionRejectsEmptyQQ(t *testing.T) {
	useTempSessionStore(t)
	if err := SaveSession(&Session{Cookie: "x"}); err == nil {
		t.Fatal("want error for empty QQ")
	}
}

// 首次使用：文件不存在时加载为空，保存后可读回（不影响首次扫码登录流程）。
func TestFirstUseFlowStillWorks(t *testing.T) {
	useTempSessionStore(t)
	sessions, err := LoadSessions()
	if err != nil || len(sessions) != 0 {
		t.Fatalf("first use = %v, %v", sessions, err)
	}
	if HasSession() {
		t.Fatal("HasSession = true on empty store")
	}
	if err := SaveSession(&Session{QQ: "111", Cookie: "c1"}); err != nil {
		t.Fatal(err)
	}
	if !HasSession() {
		t.Fatal("HasSession = false after save")
	}
}

func TestClassifyLoginCheckError(t *testing.T) {
	biz := func(code int64, msg string) *albumAPIError {
		return &albumAPIError{page: 1, kind: albumErrBiz, httpCode: 200, hasBiz: true, bizCode: code, bizMsg: msg, body: "shine_Callback({})"}
	}

	cases := []struct {
		name      string
		ctx       context.Context
		err       error
		invalid   bool
		temporary bool
	}{
		{name: "nil is valid", ctx: context.Background(), err: nil},
		{name: "biz -3000 invalid", ctx: context.Background(), err: biz(-3000, "请先登录"), invalid: true},
		{name: "biz -4001 invalid", ctx: context.Background(), err: biz(-4001, "登录已失效"), invalid: true},
		{name: "biz -87998 invalid", ctx: context.Background(), err: biz(-87998, ""), invalid: true},
		{name: "explicit login message invalid", ctx: context.Background(), err: biz(-99999, "请先登录空间"), invalid: true},
		{
			name:    "login html page invalid",
			ctx:     context.Background(),
			err:     &albumAPIError{kind: albumErrParse, httpCode: 200, body: `<!doctype html><html><script src="https://xui.ptlogin2.qq.com/xlogin"></script></html>`, cause: errors.New("invalid JSONP response")},
			invalid: true,
		},
		{
			name:      "unknown biz code is temporary",
			ctx:       context.Background(),
			err:       biz(-1, "操作过于频繁，请稍后再试"),
			temporary: true,
		},
		{
			name:      "http 429 temporary",
			ctx:       context.Background(),
			err:       &albumAPIError{kind: albumErrStatus, httpCode: 429},
			temporary: true,
		},
		{
			name:      "http 500 temporary",
			ctx:       context.Background(),
			err:       &albumAPIError{kind: albumErrStatus, httpCode: 500},
			temporary: true,
		},
		{
			name:      "http 503 temporary",
			ctx:       context.Background(),
			err:       &albumAPIError{kind: albumErrStatus, httpCode: 503},
			temporary: true,
		},
		{
			name:      "http 403 temporary",
			ctx:       context.Background(),
			err:       &albumAPIError{kind: albumErrStatus, httpCode: 403},
			temporary: true,
		},
		{
			name:      "network failure temporary",
			ctx:       context.Background(),
			err:       &albumAPIError{kind: albumErrNetwork, cause: errors.New("dial tcp: connection refused")},
			temporary: true,
		},
		{
			name:      "unexpected parse failure temporary",
			ctx:       context.Background(),
			err:       &albumAPIError{kind: albumErrParse, httpCode: 200, body: "shine_Callback(;;;)", cause: errors.New("invalid JSONP response")},
			temporary: true,
		},
		{
			name:      "plain network error temporary",
			ctx:       context.Background(),
			err:       errors.New("dial timeout"),
			temporary: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyLoginCheckError(tc.ctx, tc.err)
			if tc.err == nil {
				if got != nil {
					t.Fatalf("want nil, got %v", got)
				}
				return
			}
			if tc.invalid && !IsCredentialInvalid(got) {
				t.Fatalf("want credential invalid, got %T: %v", got, got)
			}
			if tc.temporary && !IsTemporaryLoginError(got) {
				t.Fatalf("want temporary, got %T: %v", got, got)
			}
		})
	}
}

// 已取消的 context 下，任何错误都必须是暂时性的，且错误链可观察到取消原因。
func TestClassifyContextCanceledPreservesCredential(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := classifyLoginCheckError(ctx, errors.New("request stopped"))
	if !IsTemporaryLoginError(got) {
		t.Fatalf("got %T: %v", got, got)
	}
	if IsCredentialInvalid(got) {
		t.Fatalf("canceled request must not invalidate credential: %v", got)
	}
	if !errors.Is(got, context.Canceled) {
		t.Fatalf("error chain should contain context.Canceled: %v", got)
	}
}

// 普通风控 HTML（没有任何登录站点特征）不能被误判成凭证失效。
func TestRiskControlHTMLIsTemporary(t *testing.T) {
	err := &albumAPIError{
		kind:     albumErrParse,
		httpCode: 200,
		body:     "<!doctype html><html><body>服务繁忙，请稍后再试</body></html>",
		cause:    errors.New("invalid JSONP response"),
	}
	got := classifyLoginCheckError(context.Background(), err)
	if !IsTemporaryLoginError(got) {
		t.Fatalf("got %T: %v", got, got)
	}
}
