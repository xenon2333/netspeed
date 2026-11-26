package benchmark

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xenon2333/netspeed/internal/engine"
)

func runUpload(t *testing.T, cfg Config, duration time.Duration) (*engine.Result, error) {
	t.Helper()
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	worker, err := UploadWorker(client, cfg)
	if err != nil {
		t.Fatalf("UploadWorker: %v", err)
	}
	return engine.Run(context.Background(), engine.Config{
		Duration: duration,
		Workers:  1,
		Interval: 20 * time.Millisecond,
	}, worker)
}

func TestUploadSendsConfiguredPayload(t *testing.T) {
	var mu sync.Mutex
	var sizes []int64
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := io.Copy(io.Discard, r.Body)
		mu.Lock()
		sizes = append(sizes, n)
		methods = append(methods, r.Method)
		mu.Unlock()
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	res, err := runUpload(t, Config{
		Target:      srv.URL,
		Method:      http.MethodPost,
		UploadSize:  2048,
		Timeout:     time.Second,
		Connections: 1,
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Bytes <= 0 {
		t.Errorf("Bytes = %d, want > 0", res.Bytes)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(sizes) == 0 {
		t.Fatal("server received no requests")
	}
	if sizes[0] != 2048 {
		t.Errorf("first body size = %d, want 2048", sizes[0])
	}
	if methods[0] != http.MethodPost {
		t.Errorf("first method = %q, want POST", methods[0])
	}
}

func TestUploadMethodPUT(t *testing.T) {
	var mu sync.Mutex
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = r.Method
		mu.Unlock()
		io.Copy(io.Discard, r.Body)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	if _, err := runUpload(t, Config{
		Target:      srv.URL,
		Method:      http.MethodPut,
		UploadSize:  512,
		Timeout:     time.Second,
		Connections: 1,
	}, 80*time.Millisecond); err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen != http.MethodPut {
		t.Errorf("method = %q, want PUT", seen)
	}
}

func TestUploadFollowsTemporaryRedirect(t *testing.T) {
	var mu sync.Mutex
	var bodyLen int64
	var method string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/dest", http.StatusTemporaryRedirect)
			return
		}
		n, _ := io.Copy(io.Discard, r.Body)
		mu.Lock()
		if n > bodyLen {
			bodyLen = n
		}
		method = r.Method
		mu.Unlock()
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	res, err := runUpload(t, Config{
		Target:      srv.URL + "/start",
		Method:      http.MethodPost,
		UploadSize:  2048,
		Timeout:     time.Second,
		Connections: 1,
	}, 120*time.Millisecond)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Bytes <= 0 {
		t.Errorf("Bytes = %d, want > 0", res.Bytes)
	}

	mu.Lock()
	defer mu.Unlock()
	if bodyLen != 2048 {
		t.Errorf("redirected body length = %d, want 2048", bodyLen)
	}
	if method != http.MethodPost {
		t.Errorf("redirected method = %q, want POST", method)
	}
}

func TestUploadRejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()

	start := time.Now()
	res, err := runUpload(t, Config{
		Target:      srv.URL,
		Method:      http.MethodPost,
		UploadSize:  512,
		Timeout:     time.Second,
		Connections: 1,
	}, 2*time.Second)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("err = %v, want HTTP 500", err)
	}
	if res != nil {
		t.Errorf("result = %+v, want nil", res)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("terminal error took %v, want fast failure", elapsed)
	}
}
