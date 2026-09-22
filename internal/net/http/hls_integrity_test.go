// HLS 完成信号测试：分片先落 .part，全部成功才原子改名；失败只留 .part。

package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// hlsFixtureServer 起一个最小 m3u8 服务：EXT-X-MAP 初始化段 + 3 个媒体分片。
// failSeg >= 0 时，该序号分片返回 500，模拟中途失败。
func hlsFixtureServer(t *testing.T, failSeg int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v.m3u8", func(w http.ResponseWriter, r *http.Request) {
		playlist := "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-MAP:URI=\"init.bin\"\n" +
			"#EXTINF:1.0,\nseg0.bin\n#EXTINF:1.0,\nseg1.bin\n#EXTINF:1.0,\nseg2.bin\n"
		fmt.Fprint(w, playlist)
	})
	mux.HandleFunc("/init.bin", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "INIT")
	})
	for i := 0; i < 3; i++ {
		idx := i
		mux.HandleFunc(fmt.Sprintf("/seg%d.bin", i), func(w http.ResponseWriter, r *http.Request) {
			if idx == failSeg {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, "SEG%d", idx)
		})
	}
	return httptest.NewServer(mux)
}

func TestDownloadHLSPartRenamedOnSuccess(t *testing.T) {
	srv := hlsFixtureServer(t, -1)
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "out.mp4")
	c := NewClient()
	res, err := c.DownloadHLS(context.Background(), srv.URL+"/v.m3u8", target, nil, nil, "out.mp4", "原视频", nil)
	if err != nil {
		t.Fatalf("DownloadHLS 失败: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("最终文件应存在: %v", err)
	}
	if string(data) != "INITSEG0SEG1SEG2" {
		t.Fatalf("拼接内容错误: %q", data)
	}
	if _, err := os.Stat(target + hlsPartExt); !os.IsNotExist(err) {
		t.Fatalf("成功后不应残留 .part，stat err=%v", err)
	}
	if got := res["path"]; got != target {
		t.Fatalf("返回 path = %v, want %s", got, target)
	}
	if got := res["filename"]; got != "out.mp4" {
		t.Fatalf("返回 filename = %v, want out.mp4", got)
	}
}

func TestDownloadHLSKeepsOnlyPartOnFailure(t *testing.T) {
	srv := hlsFixtureServer(t, 1) // seg1 返回 500
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "broken.mp4")
	c := NewClient()
	_, err := c.DownloadHLS(context.Background(), srv.URL+"/v.m3u8", target, nil, nil, "broken.mp4", "原视频", nil)
	if err == nil {
		t.Fatal("分片失败时应返回错误")
	}
	if HTTPStatus(err) != http.StatusInternalServerError {
		t.Fatalf("错误应体现分片 500 状态码，实际: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("失败时不得产生最终文件名，stat err=%v", err)
	}
	part := target + hlsPartExt
	info, err := os.Stat(part)
	if err != nil {
		t.Fatalf("失败后应保留 .part 半成品作为未完成标记: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal(".part 应包含已下载的 init+seg0 字节")
	}
}
