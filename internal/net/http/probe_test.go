// 预检体积探测与续传 sidecar 只读查询的单元测试

package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestProbeSize(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/head-only", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("期望 HEAD，实际 %s", r.Method)
		}
		w.Header().Set("Content-Length", "1234")
	})

	mux.HandleFunc("/range-only", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusMethodNotAllowed)
		case http.MethodGet:
			if r.Header.Get("Range") != "bytes=0-0" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Range", "bytes 0-0/5678")
			w.Header().Set("Content-Length", "1")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("x"))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/no-length", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// 先发送响应头再写实体，强制 chunked 传输，客户端拿不到 Content-Length。
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("abc"))
	})

	mux.HandleFunc("/dead", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient()
	ctx := context.Background()

	size, err := client.ProbeSize(ctx, srv.URL+"/head-only", nil)
	if err != nil {
		t.Fatalf("HEAD 直出长度失败: %v", err)
	}
	if size != 1234 {
		t.Fatalf("HEAD 长度错误: got %d want 1234", size)
	}

	size, err = client.ProbeSize(ctx, srv.URL+"/range-only", nil)
	if err != nil {
		t.Fatalf("HEAD 失败后应通过 Range GET 取总长: %v", err)
	}
	if size != 5678 {
		t.Fatalf("Content-Range 总长错误: got %d want 5678", size)
	}

	if _, err := client.ProbeSize(ctx, srv.URL+"/no-length", nil); err == nil {
		t.Fatal("无可靠长度时必须返回错误，不能当成 0 字节")
	}

	if _, err := client.ProbeSize(ctx, srv.URL+"/dead", nil); err == nil {
		t.Fatal("HEAD 与 Range GET 均 404 时必须返回错误")
	}

	if _, err := client.ProbeSize(ctx, "", nil); err == nil {
		t.Fatal("空 URL 必须报错")
	}
}

func TestResumeSidecar(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "VID_20210101_010101_abcd1234.mp4")

	// 尚不存在：全部零值。
	if size, uri, ok := ResumeSidecar(target); size != 0 || uri != "" || ok {
		t.Fatalf("不存在的文件应返回零值, got (%d,%q,%v)", size, uri, ok)
	}

	// 只有半成品，没有 sidecar：返回本地字节数但 hasURI=false（下载时会作废重下）。
	if err := os.WriteFile(target, make([]byte, 300), 0o644); err != nil {
		t.Fatal(err)
	}
	size, uri, ok := ResumeSidecar(target)
	if size != 300 || uri != "" || ok {
		t.Fatalf("无 sidecar 状态错误: got (%d,%q,%v)", size, uri, ok)
	}

	// 半成品 + 有效 sidecar：能判定按同一 URL 续传。
	meta := fmt.Sprintf(`{"uri":"https://cdn.example.com/a.mp4","updated_at":"2026-09-21T00:00:00Z"}`)
	if err := os.WriteFile(target+".resume.json", []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	size, uri, ok = ResumeSidecar(target)
	if size != 300 || !ok || uri != "https://cdn.example.com/a.mp4" {
		t.Fatalf("有 sidecar 状态错误: got (%d,%q,%v)", size, uri, ok)
	}
}
