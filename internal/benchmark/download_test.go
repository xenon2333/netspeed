package benchmark

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xenon2333/netspeed/internal/engine"
)

func runDownload(t *testing.T, cfg Config, duration time.Duration) (*engine.Result, error) {
	t.Helper()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return engine.Run(context.Background(), engine.Config{
		Duration: duration,
		Workers:  1,
		Interval: 20 * time.Millisecond,
	}, DownloadWorker(client, cfg))
}

func TestDownloadRepeatsFiniteFile(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write(make([]byte, 4096))
	}))
	defer srv.Close()

	res, err := runDownload(t, Config{
		Target:      srv.URL,
		Method:      http.MethodGet,
		BufferSize:  1024,
		Timeout:     time.Second,
		Connections: 1,
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Bytes <= 4096 {
		t.Errorf("Bytes = %d, want > 4096", res.Bytes)
	}
	if got := requests.Load(); got < 2 {
		t.Errorf("requests = %d, want >= 2", got)
	}
}

func TestDownloadRejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer srv.Close()

	start := time.Now()
	res, err := runDownload(t, Config{
		Target:      srv.URL,
		Method:      http.MethodGet,
		BufferSize:  1024,
		Timeout:     time.Second,
		Connections: 1,
	}, 2*time.Second)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want HTTP 404", err)
	}
	if res != nil {
		t.Errorf("result = %+v, want nil", res)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("terminal error took %v, want fast failure", elapsed)
	}
}

func TestDownloadRetriesTransientError(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					conn.Close()
				}
			}
			return
		}
		w.Write(make([]byte, 1024))
	}))
	defer srv.Close()

	res, err := runDownload(t, Config{
		Target:      srv.URL,
		Method:      http.MethodGet,
		BufferSize:  512,
		Timeout:     time.Second,
		Connections: 1,
	}, 600*time.Millisecond)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Bytes <= 0 {
		t.Errorf("Bytes = %d, want > 0 after retry", res.Bytes)
	}
	if got := requests.Load(); got < 2 {
		t.Errorf("requests = %d, want >= 2", got)
	}
}

func TestDownloadHeaders(t *testing.T) {
	var mu sync.Mutex
	var userAgent, custom string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		userAgent = r.Header.Get("User-Agent")
		custom = r.Header.Get("X-Test")
		mu.Unlock()
		w.Write(make([]byte, 1024))
	}))
	defer srv.Close()

	_, err := runDownload(t, Config{
		Target:      srv.URL,
		Method:      http.MethodGet,
		Headers:     []string{"X-Test: yes"},
		UserAgent:   "netspeed-test",
		BufferSize:  512,
		Timeout:     time.Second,
		Connections: 1,
	}, 60*time.Millisecond)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if userAgent != "netspeed-test" {
		t.Errorf("User-Agent = %q, want netspeed-test", userAgent)
	}
	if custom != "yes" {
		t.Errorf("X-Test = %q, want yes", custom)
	}
}

func TestDownloadTLSVerification(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, 1024))
	}))
	defer srv.Close()

	cfg := Config{
		Target:      srv.URL,
		Method:      http.MethodGet,
		BufferSize:  512,
		Timeout:     time.Second,
		Connections: 1,
	}
	if _, err := runDownload(t, cfg, 300*time.Millisecond); err == nil {
		t.Fatal("self-signed certificate accepted, want verification error")
	}

	cfg.Insecure = true
	res, err := runDownload(t, cfg, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Run with insecure: %v", err)
	}
	if res.Bytes <= 0 {
		t.Errorf("Bytes = %d, want > 0", res.Bytes)
	}
}
