package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDownloadHandlerSize(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/download?size=5000", nil)
	rr := httptest.NewRecorder()
	Handler(io.Discard).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Body.Len(); got != 5000 {
		t.Errorf("body length = %d, want 5000", got)
	}
	if got := rr.Header().Get("Content-Length"); got != "5000" {
		t.Errorf("Content-Length = %q, want 5000", got)
	}
}

func TestDownloadHandlerRejectsInvalidSize(t *testing.T) {
	for _, size := range []string{"abc", "-1", "1.5"} {
		t.Run(size, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/download?size="+size, nil)
			rr := httptest.NewRecorder()
			Handler(io.Discard).ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rr.Code)
			}
		})
	}
}

func TestDownloadHandlerStopsWhenClientGoesAway(t *testing.T) {
	handler := Handler(io.Discard)
	req := httptest.NewRequest(http.MethodGet, "/download", nil)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rr, req)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler kept streaming after client disconnected")
	}
	if rr.Body.Len() != 0 {
		t.Errorf("body length = %d, want 0", rr.Body.Len())
	}
}

func TestUploadHandler(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/upload", strings.NewReader("hello"))
			rr := httptest.NewRecorder()
			Handler(io.Discard).ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			var got struct {
				Bytes int64 `json:"bytes"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.Bytes != 5 {
				t.Errorf("bytes = %d, want 5", got.Bytes)
			}
		})
	}
}

func TestUploadHandlerRejectsGet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/upload", nil)
	rr := httptest.NewRecorder()
	Handler(io.Discard).ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
	if allow := rr.Header().Get("Allow"); !strings.Contains(allow, http.MethodPost) {
		t.Errorf("Allow = %q, want POST", allow)
	}
}

func TestRootHandler(t *testing.T) {
	rr := httptest.NewRecorder()
	Handler(io.Discard).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "/download") {
		t.Errorf("root body missing endpoint list:\n%s", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	Handler(io.Discard).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("missing path status = %d, want 404", rr.Code)
	}
}

func TestAccessLog(t *testing.T) {
	var log bytes.Buffer
	rr := httptest.NewRecorder()
	Handler(&log).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/download?size=10", nil))

	line := log.String()
	for _, want := range []string{"GET", "/download?size=10", "200", "10B"} {
		if !strings.Contains(line, want) {
			t.Errorf("access log %q missing %q", line, want)
		}
	}
}

func TestListenAddress(t *testing.T) {
	cases := map[string]string{
		"9898":           ":9898",
		"127.0.0.1:9898": "127.0.0.1:9898",
		"[::1]:9898":     "[::1]:9898",
	}
	for in, want := range cases {
		if got := listenAddress(in); got != want {
			t.Errorf("listenAddress(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestServeGracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, io.Discard) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/download?size=100")
	if err != nil {
		cancel()
		t.Fatalf("GET: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) != 100 {
		t.Errorf("body length = %d, want 100", len(body))
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not shut down")
	}
}

func TestServeShutdownClosesActiveTransfer(t *testing.T) {
	old := shutdownTimeout
	shutdownTimeout = 100 * time.Millisecond
	defer func() { shutdownTimeout = old }()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, io.Discard) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/download")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.ReadFull(resp.Body, make([]byte, 4096)); err != nil {
		t.Fatalf("read body: %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not shut down with an active transfer")
	}
}

func TestRunRejectsInvalidListen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := Run(ctx, Config{Listen: "not-an-address"}); err == nil {
		t.Fatal("Run(invalid listen) succeeded, want error")
	}
}
