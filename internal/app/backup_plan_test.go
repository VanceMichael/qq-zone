// 备份预检（BackupPlan）分类、冻结清单与零写入保证的单元测试

package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	ihttp "github.com/qinjintian/qq-zone/internal/net/http"
	"github.com/qinjintian/qq-zone/internal/pkg/util"
	"github.com/qinjintian/qq-zone/internal/qzone"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const (
	planTestUin       = "100001"
	planTestAlbumID   = "a1"
	planTestAlbumName = "Album A"
	planTestUpload    = "2021-03-04 05:06:07"
	planTestSize      = 1000
)

// newPlanTestServer 提供：HEAD 声明 1000 字节的 /img.jpg、404 的 /dead、可完整 GET 的图片实体。
func newPlanTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/img.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", planTestSize))
		if r.Method == http.MethodGet {
			_, _ = w.Write(make([]byte, planTestSize))
		}
	})
	mux.HandleFunc("/dead", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	// HEAD 一律拒绝（cookie 头与下载头都试），只有 Range GET 返回总长。
	// 用来锁定：没有 HEAD 依据时即使本地等长也不能判「完整跳过」（下载层会作废重下）。
	mux.HandleFunc("/head-denied", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Range", "bytes 0-0/1000")
		w.Header().Set("Content-Length", "1")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("x"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newPlanTestSpider(t *testing.T, groupID string) *Spider {
	t.Helper()
	client := &qzone.Client{QQ: planTestUin, Http: ihttp.NewClient()}
	cfg := &Config{TaskLimit: 4, EnableTimeline: false, EnableMetadataExport: false}
	if groupID != "" {
		return NewGroupSpider(client, cfg, nil, groupID, "测试群", zap.NewNop().Sugar())
	}
	return NewSpider(client, cfg, nil, zap.NewNop().Sugar())
}

func planPhoto(sloc, raw string) gjson.Result {
	return gjson.Parse(fmt.Sprintf(`{"sloc":%q,"raw":%q,"uploadtime":%q}`, sloc, raw, planTestUpload))
}

func planAlbum() gjson.Result {
	return gjson.Parse(fmt.Sprintf(`{"id":%q,"name":%q,"allowAccess":1}`, planTestAlbumID, planTestAlbumName))
}

// classifyOne 走单项预检分类（exclude 控制增量），由各用例按需准备本地目录。
func classifyOne(t *testing.T, s *Spider, photo gjson.Result, exclude bool) PlanItem {
	t.Helper()
	albumPath := s.plannedMediaDir(planTestUin, planTestAlbumName)
	var index map[string]string
	if exclude {
		index = readOnlyMediaIndex(albumPath)
	}
	return s.planOneItem(context.Background(), planTestUin, planAlbum(), photo, index, exclude)
}

func TestPlanItemNewDownload(t *testing.T) {
	t.Chdir(t.TempDir())
	srv := newPlanTestServer(t)
	s := newPlanTestSpider(t, "")

	item := classifyOne(t, s, planPhoto("s1", srv.URL+"/img.jpg"), true)
	if item.Kind != PlanItemNew || item.RemoteSize != planTestSize || item.Remaining != planTestSize {
		t.Fatalf("新下载分类错误: %+v", item)
	}
}

func TestPlanItemCompleteSkip(t *testing.T) {
	t.Chdir(t.TempDir())
	srv := newPlanTestServer(t)
	s := newPlanTestSpider(t, "")
	photo := planPhoto("s2", srv.URL+"/img.jpg")
	predicted := s.buildMediaSpec(photo).imageName

	albumPath := s.plannedMediaDir(planTestUin, planTestAlbumName)
	if err := os.MkdirAll(albumPath, 0o755); err != nil {
		t.Fatal(err)
	}
	// 本地文件已与远端等长，增量模式应判完整跳过、计 0 字节。
	if err := os.WriteFile(filepath.Join(albumPath, predicted), make([]byte, planTestSize), 0o644); err != nil {
		t.Fatal(err)
	}

	item := classifyOne(t, s, photo, true)
	if item.Kind != PlanItemSkip || item.Remaining != 0 || item.LocalSize != planTestSize {
		t.Fatalf("完整跳过分类错误: %+v", item)
	}
}

func TestPlanItemResume(t *testing.T) {
	t.Chdir(t.TempDir())
	srv := newPlanTestServer(t)
	s := newPlanTestSpider(t, "")
	photo := planPhoto("s3", srv.URL+"/img.jpg")
	predicted := s.buildMediaSpec(photo).imageName

	albumPath := s.plannedMediaDir(planTestUin, planTestAlbumName)
	if err := os.MkdirAll(albumPath, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(albumPath, predicted)
	if err := os.WriteFile(target, make([]byte, 400), 0o644); err != nil {
		t.Fatal(err)
	}
	// sidecar 与最高优先级候选同一 URL：可续传，只计尚未写入的 600 字节。
	meta := fmt.Sprintf(`{"uri":%q}`, srv.URL+"/img.jpg")
	if err := os.WriteFile(target+".resume.json", []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	item := classifyOne(t, s, photo, true)
	if item.Kind != PlanItemResume || item.Remaining != 600 || item.LocalSize != 400 || item.RemoteSize != 1000 {
		t.Fatalf("可续传分类错误: %+v", item)
	}
}

func TestPlanItemPartialWithoutSidecarIsNew(t *testing.T) {
	t.Chdir(t.TempDir())
	srv := newPlanTestServer(t)
	s := newPlanTestSpider(t, "")
	photo := planPhoto("s4", srv.URL+"/img.jpg")
	predicted := s.buildMediaSpec(photo).imageName

	albumPath := s.plannedMediaDir(planTestUin, planTestAlbumName)
	if err := os.MkdirAll(albumPath, 0o755); err != nil {
		t.Fatal(err)
	}
	// 有半成品但 sidecar 缺失：下载层会先删后重下，预检必须按整份新下载计 1000 字节。
	if err := os.WriteFile(filepath.Join(albumPath, predicted), make([]byte, 400), 0o644); err != nil {
		t.Fatal(err)
	}

	item := classifyOne(t, s, photo, true)
	if item.Kind != PlanItemNew || item.Remaining != planTestSize {
		t.Fatalf("无 sidecar 半成品应按新下载计整份字节: %+v", item)
	}
}

func TestPlanItemHeadFailurePreventsSkip(t *testing.T) {
	t.Chdir(t.TempDir())
	srv := newPlanTestServer(t)
	s := newPlanTestSpider(t, "")
	photo := planPhoto("s2b", srv.URL+"/head-denied")
	predicted := s.buildMediaSpec(photo).imageName

	albumPath := s.plannedMediaDir(planTestUin, planTestAlbumName)
	if err := os.MkdirAll(albumPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(albumPath, predicted), make([]byte, planTestSize), 0o644); err != nil {
		t.Fatal(err)
	}

	// 与 shouldSkipExisting 一致：HEAD 拿不到长度就不能判跳过；真实执行会作废半成品整份重下，
	// 预检必须按新下载 1000 字节计，而不是跳过计 0。
	item := classifyOne(t, s, photo, true)
	if item.Kind != PlanItemNew || item.Remaining != planTestSize {
		t.Fatalf("HEAD 不可用时不能跳过，应按新下载计整份: %+v", item)
	}
}

func TestPlanItemProbeFailureIsUnknown(t *testing.T) {
	t.Chdir(t.TempDir())
	srv := newPlanTestServer(t)
	s := newPlanTestSpider(t, "")

	item := classifyOne(t, s, planPhoto("s5", srv.URL+"/dead"), true)
	if item.Kind != PlanItemUnknown || item.Remaining != 0 || item.Reason == "" {
		t.Fatalf("探测失败必须列为未知且不计字节: %+v", item)
	}
}

func TestPlanItemNonIncrementalIgnoresLocal(t *testing.T) {
	t.Chdir(t.TempDir())
	srv := newPlanTestServer(t)
	s := newPlanTestSpider(t, "")
	photo := planPhoto("s6", srv.URL+"/img.jpg")
	predicted := s.buildMediaSpec(photo).imageName

	albumPath := s.plannedMediaDir(planTestUin, planTestAlbumName)
	if err := os.MkdirAll(albumPath, 0o755); err != nil {
		t.Fatal(err)
	}
	// 本地已有完整文件，但非增量模式执行时会清空重下，预检必须按新下载计整份。
	if err := os.WriteFile(filepath.Join(albumPath, predicted), make([]byte, planTestSize), 0o644); err != nil {
		t.Fatal(err)
	}

	item := classifyOne(t, s, photo, false)
	if item.Kind != PlanItemNew || item.Remaining != planTestSize {
		t.Fatalf("非增量模式必须按整份新下载计: %+v", item)
	}
}

func TestPlanItemHLSAlwaysUnknown(t *testing.T) {
	t.Chdir(t.TempDir())
	s := newPlanTestSpider(t, "123456")
	photo := gjson.Parse(`{"sloc":"gv1","is_video":true,"videodata":{"actionurl":"https://vod.example.com/a.m3u8"}}`)

	item := classifyOne(t, s, photo, true)
	if item.Kind != PlanItemUnknown || item.Remaining != 0 {
		t.Fatalf("仅 HLS 源必须列为未知且不能当 0 字节跳过: %+v", item)
	}
}

func TestBuildBackupPlanFreezesAndWritesNothing(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	srv := newPlanTestServer(t)
	s := newPlanTestSpider(t, "")

	listCalls := 0
	s.planListPhotos = func(ctx context.Context, targetUin, groupID, albumID string) ([]gjson.Result, error) {
		listCalls++
		return []gjson.Result{
			planPhoto("f1", srv.URL+"/img.jpg"),
			planPhoto("f2", srv.URL+"/img.jpg"),
		}, nil
	}

	albumRoot := filepath.Join("storage", "qzone", planTestUin, "album")
	if _, err := os.Stat(albumRoot); !os.IsNotExist(err) {
		t.Fatalf("预检前相册根目录不应存在")
	}

	plan, err := s.BuildBackupPlan(context.Background(), planTestUin, []gjson.Result{planAlbum()}, true)
	if err != nil {
		t.Fatalf("构建预检计划失败: %v", err)
	}
	if plan.Total != 2 || plan.NewCount != 2 || plan.KnownRemaining != 2*planTestSize {
		t.Fatalf("计划计数错误: total=%d new=%d bytes=%d", plan.Total, plan.NewCount, plan.KnownRemaining)
	}
	if plan.FreeBytes <= 0 || plan.VolumePath == "" {
		t.Fatalf("目标卷容量必须被填充: free=%d mount=%q", plan.FreeBytes, plan.VolumePath)
	}
	if len(plan.Albums) != 1 || len(plan.Albums[0].Photos) != 2 {
		t.Fatal("冻结清单必须包含两个相册项的完整媒体列表")
	}
	if listCalls != 1 {
		t.Fatalf("预检期间每个相册应只拉一次清单, got %d", listCalls)
	}
	// 预检零写入：相册根目录仍不能被创建。
	if _, err := os.Stat(albumRoot); !os.IsNotExist(err) {
		t.Fatal("预检结束后不应创建任何相册目录/文件")
	}

	// 执行阶段只能消费冻结清单：不得再次触发分页拉取。
	results, runErr := s.DownloadPlan(context.Background(), planTestUin, plan)
	if runErr != nil {
		t.Fatalf("执行冻结计划失败: %v", runErr)
	}
	if listCalls != 1 {
		t.Fatalf("执行阶段重新拉取了清单（%d 次），冻结保证被破坏", listCalls)
	}
	if results.Total != 2 || results.Success != 2 || results.NewAdded != 2 {
		t.Fatalf("冻结计划执行结果错误: %+v", results)
	}

	// 两个文件都真实落盘（时间线关闭，直接在相册目录下）。
	albumPath := s.plannedMediaDir(planTestUin, planTestAlbumName)
	got, err := util.ListFiles(albumPath)
	if err != nil || len(got) != 2 {
		t.Fatalf("应有 2 个文件落盘, got %d, err=%v", len(got), err)
	}
	for _, f := range got {
		if fi, statErr := os.Stat(f); statErr != nil || fi.Size() != planTestSize {
			t.Fatalf("落盘文件大小错误: %s %v", f, statErr)
		}
	}
}

func TestBuildBackupPlanPaginationFailureInvalidatesPlan(t *testing.T) {
	t.Chdir(t.TempDir())
	s := newPlanTestSpider(t, "")
	s.planListPhotos = func(ctx context.Context, targetUin, groupID, albumID string) ([]gjson.Result, error) {
		return nil, fmt.Errorf("connection reset by peer")
	}

	plan, err := s.BuildBackupPlan(context.Background(), planTestUin, []gjson.Result{planAlbum()}, true)
	if err == nil || plan != nil {
		t.Fatalf("分页中途失败必须返回错误并作废整份计划: plan=%v err=%v", plan, err)
	}
	// 零写入保证。
	if _, statErr := os.Stat(filepath.Join("storage", "qzone", planTestUin)); !os.IsNotExist(statErr) {
		t.Fatalf("分页失败后不应留下任何目录: %v", statErr)
	}
}

func TestBackupPlanSpaceShortage(t *testing.T) {
	plan := &BackupPlan{KnownRemaining: 100, FreeBytes: 99}
	if !plan.SpaceShortage() {
		t.Fatal("已知待写大于可用空间时必须判定不足")
	}
	plan.FreeBytes = 100
	if plan.SpaceShortage() {
		t.Fatal("容量刚好够时不应判定不足")
	}
}
