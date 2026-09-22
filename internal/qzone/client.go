// QQ 空间核心 API 客户端，封装相册、照片及视频下载地址的获取逻辑

package qzone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/qinjintian/qq-zone/internal/net/http"
	"github.com/qinjintian/qq-zone/internal/pkg/logger"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// Client 封装了与 QQ 空间 API 交互的客户端
// 包含 HTTP 客户端实例、用户凭证 (Cookie/GTK) 以及日志记录器
type Client struct {
	QQ        string             // 当前登录用户的 QQ 号码
	Nickname  string             // 当前登录用户的昵称
	GTK       string             // 算出的 QQ 空间 CSRF 校验 Token (g_tk)
	Cookie    string             // QQ 空间接口 Cookie
	QunCookie string             // 群管理页 Cookie；有 p_skey 才能列出加入的群
	Http      *http.Client       // 底层 HTTP 客户端，负责发送网络请求
	APILogger *zap.SugaredLogger // 专门用于记录 API 请求和响应的日志记录器
}

// NewClient 初始化并返回一个 QQ 空间 API 客户端实例 (优先尝试 Session，仅在凭证被明确判定失效时才扫码登录)
func NewClient(ctx context.Context, httpClient *http.Client, logFact *logger.Factory) (*Client, error) {
	// 1. 尝试从本地加载最近使用的 Session；存储故障必须直接上报，不能当成空库去扫码并覆盖原文件。
	sess, err := GetLastSession()
	if err != nil {
		return nil, err
	}
	if sess != nil {
		c, err := NewClientWithSession(ctx, sess, httpClient, logFact)
		if err == nil {
			return c, nil
		}
		// 只有 QQ 空间明确判定凭证失效时，才退回扫码登录；
		// 暂时故障（取消/网络/限流/5xx/解析失败）或存储错误都要保留账号并把错误交出去。
		if !IsCredentialInvalid(err) {
			return nil, err
		}
	}
	// 2. 不存在历史账号或凭证明确失效，执行新登录流程
	return NewClientWithQR(ctx, httpClient, logFact)
}

// NewClientWithSession 从已有的 Session 实例化一个客户端实例，并在线校验登录态。
// 仅当 QQ 空间明确返回凭证失效时才删除该账号；context 取消、网络失败、限流、5xx、
// 非预期响应或解析失败一律保留原凭证并返回可观察错误，由调用方提示稍后重试或重选账号。
// 校验通过后只在持久化成功时刷新最近使用时间，群授权和其他账号的内容原样保留。
func NewClientWithSession(ctx context.Context, sess *Session, httpClient *http.Client, logFact *logger.Factory) (*Client, error) {
	apiLogger, _ := logFact.CreateAPILogger(sess.QQ)
	c := &Client{
		QQ:        sess.QQ,
		Nickname:  sess.Nickname,
		GTK:       sess.GTK,
		Cookie:    sess.Cookie,
		QunCookie: sess.QunCookie,
		Http:      httpClient,
		APILogger: apiLogger,
	}

	if err := c.CheckLogin(ctx); err != nil {
		if IsCredentialInvalid(err) {
			// QQ 空间明确判定凭证失效：只删除当前选中的这一个账号，再引导用户重新扫码。
			if rmErr := RemoveSession(sess.QQ); rmErr != nil {
				return nil, fmt.Errorf("%w；同时本地失效账号删除失败，已停止以保护会话文件: %v", err, rmErr)
			}
			return nil, err
		}
		// 其它失败一律保留原凭证，绝不能删账号或弹出新二维码。
		return nil, err
	}

	// 校验通过：仅刷新最近使用时间（群授权、其他账号全部保留）。
	// 持久化失败时必须返回错误，旧文件已原子保留，不能伪装成已完成登录。
	if err := TouchSession(sess.QQ); err != nil {
		return nil, fmt.Errorf("账号凭证校验通过，但本地会话刷新失败（旧文件已保留）: %w", err)
	}
	return c, nil
}

// NewClientWithQR 发起二维码登录流程
// 它会获取登录二维码，保存到本地，轮询登录状态，最后解析凭证并初始化客户端信息
func NewClientWithQR(ctx context.Context, httpClient *http.Client, logFact *logger.Factory) (*Client, error) {
	loginRes, err := NewLoginHandler(httpClient).Login(ctx)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	cookie := loginRes["cookie"]
	qq := qqFromCookie(cookie)

	apiLogger, _ := logFact.CreateAPILogger(qq)
	qunCookie := qunCookieForQQ(qq)

	c := &Client{
		QQ:        qq,
		Nickname:  loginRes["nickname"],
		GTK:       loginRes["g_tk"],
		Cookie:    cookie,
		QunCookie: qunCookie,
		Http:      httpClient,
		APILogger: apiLogger,
	}

	// 只有会话成功落盘才算完成登录；存储故障（如会话文件损坏）时必须报错，
	// 不能伪装成登录成功。
	if err := c.persistSession(); err != nil {
		return nil, fmt.Errorf("登录成功，但本地会话保存失败，未完成登录（旧会话文件已保留）: %w", err)
	}

	return c, nil
}

// NewClientFromSession 通过已有的 Session 会话直接恢复客户端状态（不进行有效性校验）
func NewClientFromSession(sess *Session, httpClient *http.Client, apiLogger *zap.SugaredLogger) *Client {
	return &Client{
		QQ:        sess.QQ,
		Nickname:  sess.Nickname,
		GTK:       sess.GTK,
		Cookie:    sess.Cookie,
		QunCookie: sess.QunCookie,
		Http:      httpClient,
		APILogger: apiLogger,
	}
}

// HasQunAuth 是否已经有群管理页的 p_skey，能用来拉加入的群。
func (c *Client) HasQunAuth() bool {
	return c != nil && extractCookieValue(c.QunCookie, "p_skey") != ""
}

// ClearQunAuth 丢掉过期的群列表授权，空间登录不受影响。
// 内存态无论是否落盘成功都会清掉；返回 error 表示本地文件没能同步，旧文件保持原样。
func (c *Client) ClearQunAuth() error {
	if c == nil {
		return nil
	}
	c.QunCookie = ""
	return c.persistSession()
}

// AuthorizeQun 弹出群管理页二维码。必须用当前空间登录的同一个 QQ。
func (c *Client) AuthorizeQun(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("尚未登录空间")
	}
	res, err := NewLoginHandler(c.Http).LoginQun(ctx)
	if err != nil {
		return err
	}
	cookie := res["cookie"]
	qq := qqFromCookie(cookie)
	if qq != "" && c.QQ != "" && qq != c.QQ {
		return fmt.Errorf("刚才扫的是 %s，当前空间登录是 %s。请用同一个号再扫一次", qq, c.QQ)
	}
	if extractCookieValue(cookie, "p_skey") == "" {
		return fmt.Errorf("没有拿到群列表凭证，请再扫一次")
	}
	c.QunCookie = cookie
	// 授权在当前进程内已生效；落盘失败必须告诉调用方，不能假装下次也免扫码。
	if err := c.persistSession(); err != nil {
		return fmt.Errorf("群授权成功，但本地会话保存失败（旧会话文件已保留，下次可能需要重新授权）: %w", err)
	}
	return nil
}

func (c *Client) persistSession() error {
	if c == nil || strings.TrimSpace(c.QQ) == "" {
		return nil
	}
	return SaveSession(&Session{
		QQ:        c.QQ,
		Nickname:  c.Nickname,
		GTK:       c.GTK,
		Cookie:    c.Cookie,
		QunCookie: c.QunCookie,
	})
}

// CredentialInvalidError 表示 QQ 空间明确返回登录凭证已失效：
// 要么是明确的登录失效业务码，要么是被跳转到了登录页。
// 只有命中这一类错误，才允许删除本地账号并引导重新扫码。
type CredentialInvalidError struct {
	Reason string
}

func (e *CredentialInvalidError) Error() string {
	if e == nil || e.Reason == "" {
		return "QQ 空间登录凭证已失效，需要重新扫码登录"
	}
	return "QQ 空间登录凭证已失效（" + e.Reason + "），需要重新扫码登录"
}

// IsCredentialInvalid 判断错误是否为“空间明确判定凭证失效”。
func IsCredentialInvalid(err error) bool {
	var target *CredentialInvalidError
	return errors.As(err, &target)
}

// TemporaryLoginError 表示登录态校验暂时无法完成：
// context 取消/超时、网络失败、限流、5xx、非预期响应或解析失败。
// 凭证并未被判定失效，本地记录必须原样保留，用户可以稍后重试或重选账号。
type TemporaryLoginError struct {
	Reason string
	Cause  error
}

func (e *TemporaryLoginError) Error() string {
	if e == nil {
		return "登录态校验暂时无法完成"
	}
	if e.Cause != nil {
		return "登录态校验暂时无法完成（" + e.Reason + "）: " + e.Cause.Error()
	}
	return "登录态校验暂时无法完成（" + e.Reason + "），本地凭证已保留，请稍后重试或重选账号"
}

func (e *TemporaryLoginError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// IsTemporaryLoginError 判断错误是否为“暂时无法校验、凭证已保留”。
func IsTemporaryLoginError(err error) bool {
	var target *TemporaryLoginError
	return errors.As(err, &target)
}

// CheckLogin 用一次相册列表请求在线校验当前凭证是否仍被 QQ 空间认可。
// 返回 nil 表示有效；返回 CredentialInvalidError 表示空间明确要求重新登录；
// 其它情况一律包装成 TemporaryLoginError，调用方必须保留本地凭证。
func (c *Client) CheckLogin(ctx context.Context) error {
	_, err := c.GetAlbumList(ctx, c.QQ)
	return classifyLoginCheckError(ctx, err)
}

// loginExpiredBizCodes 是 QQ 空间各 CGI 明确表示“登录已失效/请先登录”的业务码，
// 与 moodAPIError / boardAPIError 中的判定保持一致。
var loginExpiredBizCodes = map[int64]struct{}{
	-3000:  {},
	-4001:  {},
	-87998: {},
}

// loginExpiredKeywords 是空间业务消息里明确要求重新登录的文案。
var loginExpiredKeywords = []string{
	"请先登录",
	"未登录",
	"登录已失效",
	"登录态失效",
	"登录态过期",
	"登录过期",
	"需要登录",
	"重新登录",
}

// classifyLoginCheckError 把相册列表请求的失败分成三类：
// 明确失效（CredentialInvalidError）、暂时故障（TemporaryLoginError）和成功（nil）。
// 默认按暂时故障处理，宁可让用户稍后重试，也不能误删仍然有效的凭证。
func classifyLoginCheckError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	// context 取消/超时最先判定：一定是暂时故障；用 Join 保留取消原因与原始错误链。
	if ctxErr := ctx.Err(); ctxErr != nil {
		return &TemporaryLoginError{Reason: "校验被取消或等待超时", Cause: errors.Join(err, ctxErr)}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &TemporaryLoginError{Reason: "校验被取消或等待超时", Cause: err}
	}

	var ae *albumAPIError
	if errors.As(err, &ae) {
		// 1) 空间明确的登录失效业务码。
		if ae.hasBiz {
			if _, ok := loginExpiredBizCodes[ae.bizCode]; ok {
				return &CredentialInvalidError{Reason: fmt.Sprintf("空间业务码 %d: %s", ae.bizCode, ae.bizMsg)}
			}
			if containsAny(ae.bizMsg, loginExpiredKeywords) {
				return &CredentialInvalidError{Reason: ae.bizMsg}
			}
		}
		// 2) Cookie 失效后被重定向到登录页（跟随跳转后拿到的是登录页 HTML）。
		if looksLikeLoginPage(ae.body) {
			return &CredentialInvalidError{Reason: "空间响应跳转到了登录页面"}
		}
		// 3) 其余一律视为暂时故障：限流、5xx、其它 4xx、网络错误、非预期响应/解析失败。
		switch ae.kind {
		case albumErrNetwork:
			return &TemporaryLoginError{Reason: "网络请求失败或连接中断", Cause: err}
		case albumErrStatus:
			switch {
			case ae.httpCode == 429:
				return &TemporaryLoginError{Reason: "请求被限流 (HTTP 429)", Cause: err}
			case ae.httpCode >= 500:
				return &TemporaryLoginError{Reason: fmt.Sprintf("空间服务暂时异常 (HTTP %d)", ae.httpCode), Cause: err}
			default:
				return &TemporaryLoginError{Reason: fmt.Sprintf("校验请求被拒绝 (HTTP %d)", ae.httpCode), Cause: err}
			}
		default:
			// 解析失败 / 未知业务码等非预期响应：保留凭证，稍后重试。
			return &TemporaryLoginError{Reason: "空间返回了非预期响应或响应解析失败", Cause: err}
		}
	}

	// 非相册接口错误（底层网络错误等）：保守按暂时故障处理。
	return &TemporaryLoginError{Reason: "网络请求失败", Cause: err}
}

func containsAny(s string, keywords []string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// looksLikeLoginPage 判断响应体是否为 QQ 登录页 HTML。仅在确认是 HTML 且包含
// 登录站点特征时才判定为凭证失效，避免把普通风控页误判成失效。
func looksLikeLoginPage(body string) bool {
	if !cgiLooksLikeHTML(body) {
		return false
	}
	lower := strings.ToLower(body)
	if strings.Contains(lower, "ptlogin") ||
		strings.Contains(lower, "xui.ptlogin") ||
		strings.Contains(lower, "idlogin") ||
		strings.Contains(lower, "qrcodelogin") {
		return true
	}
	return strings.Contains(body, "请先登录") ||
		strings.Contains(body, "安全登录") ||
		strings.Contains(body, "一键登录") ||
		strings.Contains(body, "统一登录")
}

// albumAPIError 携带一次相册列表请求失败的完整上下文，供登录态分类使用，
// 同时保持与历史错误文案兼容。
type albumAPIError struct {
	page     int            // 第几页请求（从 1 开始）
	kind     albumErrorKind // 失败环节
	httpCode int            // HTTP 状态码；0 表示请求未拿到响应
	hasBiz   bool           // 是否拿到了空间业务码
	bizCode  int64          // 空间业务码
	bizMsg   string         // 空间业务消息
	body     string         // 原始响应体
	cause    error          // 底层错误（网络/解析）
}

type albumErrorKind int

const (
	albumErrNetwork albumErrorKind = iota // 请求未完成（取消/网络/限流等待）
	albumErrStatus                        // 拿到非 200 HTTP 状态
	albumErrParse                         // 响应体无法解析
	albumErrBiz                           // 空间返回非 0 业务码
)

func (e *albumAPIError) Error() string {
	if e == nil {
		return ""
	}
	switch e.kind {
	case albumErrBiz:
		return fmt.Sprintf("api error (code: %d): %s", e.bizCode, e.bizMsg)
	case albumErrParse:
		return fmt.Sprintf("failed to parse album response: %v (raw body: %s)", e.cause, e.body)
	case albumErrStatus:
		return fmt.Sprintf("failed to fetch album page %d: status code %d", e.page, e.httpCode)
	default:
		return fmt.Sprintf("failed to fetch album page %d: %v", e.page, e.cause)
	}
}

func (e *albumAPIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// logAPI 负责统一记录底层 API 请求的调试信息
// 包含请求的 URL、状态码以及原始的响应内容，方便在出现风控或错误时进行排查
func (c *Client) logAPI(apiName, url string, headers map[string]string, body string, statusCode int, duration time.Duration, err error) {
	if c.APILogger == nil {
		return
	}

	status := "SUCCESS"
	if err != nil || (statusCode != 0 && statusCode >= 400) {
		status = "FAILED"
	}

	logMsg := fmt.Sprintf("\n%s\n", strings.Repeat("=", 60))
	logMsg += fmt.Sprintf(">>> API [%s] %s <<<\n", apiName, status)
	logMsg += fmt.Sprintf("Time:     %s\n", time.Now().Format("2006-01-02 15:04:05.000"))
	logMsg += fmt.Sprintf("Duration: %v\n", duration)
	logMsg += fmt.Sprintf("Status:   %d\n", statusCode)
	logMsg += fmt.Sprintf("URL:      %s\n", url)

	logMsg += "\n[Request Headers]\n"
	for k, v := range headers {
		logMsg += fmt.Sprintf("  %s: %s\n", k, v)
	}

	if err != nil {
		logMsg += fmt.Sprintf("\n[Error]\n  %v\n", err)
	}

	if body != "" {
		logMsg += "\n[Response Body]\n"
		// 尝试格式化 JSON 以便阅读
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, []byte(body), "", "  "); err == nil {
			logMsg += prettyJSON.String()
		} else {
			// 如果不是标准 JSON（如 JSONP），则直接输出
			logMsg += body
		}
	}
	logMsg += "\n" + strings.Repeat("=", 60) + "\n"

	if err != nil || (statusCode != 0 && statusCode >= 400) {
		c.APILogger.Errorf(logMsg)
	} else {
		c.APILogger.Debug(logMsg)
	}
}

// GetAlbumList 拉取指定 QQ 号的相册列表
// 返回一个包含所有相册信息的 gjson.Result 数组
func (c *Client) GetAlbumList(ctx context.Context, targetUin string) ([]gjson.Result, error) {
	headers := map[string]string{
		"cookie":     c.Cookie,
		"user-agent": UserAgent,
		"referer":    fmt.Sprintf("https://user.qzone.qq.com/%s/infocenter", c.QQ),
		"origin":     "https://user.qzone.qq.com",
	}

	var (
		offset    int64 = 0
		limit     int64 = 30
		allAlbums []gjson.Result
	)

	for {
		apiURL := fmt.Sprintf("https://user.qzone.qq.com/proxy/domain/photo.qzone.qq.com/fcgi-bin/fcg_list_album_v3?g_tk=%v&callback=shine_Callback&hostUin=%v&uin=%v&appid=4&inCharset=utf-8&outCharset=utf-8&source=qzone&plat=qzone&format=jsonp&notice=0&filter=1&handset=4&pageNumModeSort=40&pageNumModeClass=15&needUserInfo=1&idcNum=4&mode=2&pageStart=%d&pageNum=%d&callbackFun=shine", c.GTK, targetUin, c.QQ, offset, limit)

		start := time.Now()
		_, body, code, err := c.Http.Get(ctx, apiURL, headers)
		duration := time.Since(start)

		bodyStr := string(body)
		c.logAPI("GetAlbumList", apiURL, headers, bodyStr, code, duration, err)

		pageNo := int(offset/limit + 1)
		if err != nil {
			return nil, &albumAPIError{page: pageNo, kind: albumErrNetwork, cause: err}
		}
		if code != 200 {
			return nil, &albumAPIError{page: pageNo, kind: albumErrStatus, httpCode: code, body: bodyStr,
				cause: fmt.Errorf("status code %d", code)}
		}

		data, err := parseJSONP(bodyStr, "shine_Callback")
		if err != nil {
			return nil, &albumAPIError{page: pageNo, kind: albumErrParse, httpCode: code, body: bodyStr, cause: err}
		}

		res := gjson.Parse(data)
		if bizCode := res.Get("code").Int(); bizCode != 0 {
			msg := res.Get("message").String()
			if msg == "" {
				msg = res.Get("msg").String()
			}
			return nil, &albumAPIError{
				page:     pageNo,
				kind:     albumErrBiz,
				httpCode: code,
				hasBiz:   true,
				bizCode:  bizCode,
				bizMsg:   msg,
				body:     bodyStr,
			}
		}

		albumList := res.Get("data.albumList").Array()
		// 尝试兼容不同的 JSON 路径
		if len(albumList) == 0 && offset == 0 {
			if altList := res.Get("albumList").Array(); len(altList) > 0 {
				albumList = altList
			}
		}

		allAlbums = append(allAlbums, albumList...)

		nextPageStart := res.Get("data.nextPageStart").Int()
		totalAlbums := res.Get("data.albumsInUser").Int()

		if nextPageStart >= totalAlbums || len(albumList) == 0 {
			break
		}
		offset = nextPageStart
	}

	return allAlbums, nil
}

// GetPhotoList 分页拉取指定相册下的所有照片/视频列表
// 由于 QQ 空间接口有分页限制，此方法会自动循环请求直到拉取完所有数据
func (c *Client) GetPhotoList(ctx context.Context, targetUin string, albumID string) ([]gjson.Result, error) {
	headers := map[string]string{
		"cookie":     c.Cookie,
		"user-agent": UserAgent,
		"referer":    fmt.Sprintf("https://user.qzone.qq.com/%s/4", targetUin),
		"origin":     "https://user.qzone.qq.com",
	}

	var (
		offset     int64 = 0
		limit      int64 = 500
		allPhotos  []gjson.Result
		photoCount int64 = 0
	)

	for {
		apiURL := fmt.Sprintf("https://user.qzone.qq.com/proxy/domain/photo.qzone.qq.com/fcgi-bin/cgi_list_photo?g_tk=%v&callback=shine_Callback&mode=0&idcNum=4&hostUin=%v&topicId=%v&noTopic=0&uin=%v&pageStart=%v&pageNum=%v&skipCmtCount=0&singleurl=1&batchId=&notice=0&appid=4&inCharset=utf-8&outCharset=utf-8&source=qzone&plat=qzone&outstyle=json&format=jsonp&json_esc=1&callbackFun=shine", c.GTK, targetUin, albumID, c.QQ, offset, limit)

		start := time.Now()
		header, body, code, err := c.Http.Get(ctx, apiURL, headers)
		duration := time.Since(start)

		bodyStr := string(body)
		c.logAPI("GetPhotoList", apiURL, headers, bodyStr, code, duration, err)

		if err != nil {
			return nil, fmt.Errorf("failed to fetch photo page: %w", err)
		}
		if code != 200 {
			return nil, fmt.Errorf("failed to fetch photo page: status code %d", code)
		}

		for _, cookie := range header.Values("Set-Cookie") {
			if key := extractCookieValue(cookie, "qq_photo_key"); key != "" {
				if !strings.Contains(c.Cookie, "qq_photo_key") {
					c.Cookie += "; qq_photo_key=" + key
					headers["cookie"] = c.Cookie
				}
				break
			}
		}

		data, err := parseJSONP(bodyStr, "shine_Callback")
		if err != nil {
			return nil, fmt.Errorf("failed to parse photo response: %w (raw body: %s)", err, bodyStr)
		}

		res := gjson.Parse(data)
		if res.Get("code").Int() != 0 {
			return nil, fmt.Errorf("api error: %s", res.Get("message").String())
		}

		list := res.Get("data.photoList").Array()
		allPhotos = append(allPhotos, list...)
		photoCount += int64(len(list))

		totalInAlbum := res.Get("data.totalInAlbum").Int()
		if totalInAlbum == 0 {
			totalInAlbum = res.Get("data.total").Int()
		}

		if photoCount >= totalInAlbum || len(list) == 0 {
			break
		}
		offset += limit
	}

	return allPhotos, nil
}

// GetFriendList 获取当前用户的所有好友列表
// 并并发检测每个好友的空间访问权限 (是否对我开放)
func (c *Client) GetFriendList(ctx context.Context) ([]gjson.Result, error) {
	apiURL := fmt.Sprintf("https://user.qzone.qq.com/proxy/domain/r.qzone.qq.com/cgi-bin/tfriend/friend_ship_manager.cgi?uin=%v&do=1&fupdate=1&clean=1&g_tk=%v", c.QQ, c.GTK)
	headers := map[string]string{
		"cookie":     c.Cookie,
		"user-agent": UserAgent,
		"referer":    fmt.Sprintf("https://user.qzone.qq.com/%s/infocenter", c.QQ),
		"origin":     "https://user.qzone.qq.com",
	}

	start := time.Now()
	_, body, code, err := c.Http.Get(ctx, apiURL, headers)
	duration := time.Since(start)

	bodyStr := string(body)
	c.logAPI("GetFriendList", apiURL, headers, bodyStr, code, duration, err)

	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("failed to fetch friend list: status code %d", code)
	}

	data, err := parseJSONP(bodyStr, "shine_Callback")
	if err != nil {
		return nil, fmt.Errorf("failed to parse friend response: %w (raw body: %s)", err, bodyStr)
	}

	res := gjson.Parse(data)
	if res.Get("code").Int() != 0 {
		return nil, fmt.Errorf("api error: %s", res.Get("message").String())
	}

	return res.Get("data.items_list").Array(), nil
}

// Helper functions - 以下为内部辅助函数
// parseJSONP 用于解析腾讯接口常返回的 JSONP 格式数据
// 它会剥离外部的 callback 回调函数包装，提取并返回纯净的内部 JSON 字符串
func parseJSONP(content string, callback string) (string, error) {
	start := strings.Index(content, "(")
	end := strings.LastIndex(content, ")")
	if start == -1 || end == -1 || end <= start {
		return "", fmt.Errorf("invalid JSONP response")
	}
	return content[start+1 : end], nil
}

// GetAlbumListURL 根据给定的参数生成并返回拉取相册列表的 API URL
func GetAlbumListURL(hostUin, uin, gtk string) string {
	return fmt.Sprintf("https://user.qzone.qq.com/proxy/domain/photo.qzone.qq.com/fcgi-bin/fcg_list_album_v3?g_tk=%v&callback=shine_Callback&hostUin=%v&uin=%v&appid=4&inCharset=utf-8&outCharset=utf-8&source=qzone&plat=qzone&format=jsonp&notice=0&filter=1&handset=4&pageNumModeSort=40&pageNumModeClass=15&needUserInfo=1&idcNum=4&callbackFun=shine", gtk, hostUin, uin)
}
