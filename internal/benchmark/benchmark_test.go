package benchmark

import (
	"net/http"
	"testing"
	"time"
)

func TestDirection(t *testing.T) {
	cases := map[string]string{
		http.MethodGet:  "download",
		http.MethodPost: "upload",
		http.MethodPut:  "upload",
	}
	for method, want := range cases {
		if got := Direction(method); got != want {
			t.Errorf("Direction(%q) = %q, want %q", method, got, want)
		}
	}
}

func TestNewClientSettings(t *testing.T) {
	client, err := NewClient(Config{
		Timeout:     7 * time.Second,
		Connections: 3,
		Insecure:    true,
		NoProxy:     true,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.Timeout != 0 {
		t.Errorf("client.Timeout = %v, want 0", client.Timeout)
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if tr.ResponseHeaderTimeout != 7*time.Second {
		t.Errorf("ResponseHeaderTimeout = %v, want 7s", tr.ResponseHeaderTimeout)
	}
	if tr.TLSHandshakeTimeout != 7*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 7s", tr.TLSHandshakeTimeout)
	}
	if !tr.DisableCompression {
		t.Error("DisableCompression = false, want true")
	}
	if tr.MaxConnsPerHost != 3 || tr.MaxIdleConnsPerHost != 3 {
		t.Errorf("connection limits = %d/%d, want 3/3", tr.MaxConnsPerHost, tr.MaxIdleConnsPerHost)
	}
	if tr.Proxy != nil {
		t.Error("Proxy != nil with NoProxy, want nil")
	}
	if tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("TLSClientConfig.InsecureSkipVerify = false, want true")
	}
	if tr.ForceAttemptHTTP2 {
		t.Error("ForceAttemptHTTP2 = true, want false")
	}
	if tr.TLSNextProto == nil || len(tr.TLSNextProto) != 0 {
		t.Errorf("TLSNextProto = %v, want non-nil empty map", tr.TLSNextProto)
	}
}

func TestNewClientInvalidBind(t *testing.T) {
	if _, err := NewClient(Config{Bind: "not-an-ip"}); err == nil {
		t.Fatal("NewClient(invalid bind) succeeded, want error")
	}
}

func TestApplyHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	applyHeaders(req, []string{"X-Test: 1", "User-Agent: ua2", "Host: example.test"}, "ua1")
	if got := req.Header.Get("X-Test"); got != "1" {
		t.Errorf("X-Test = %q, want 1", got)
	}
	if got := req.Header.Get("User-Agent"); got != "ua2" {
		t.Errorf("User-Agent = %q, want ua2", got)
	}
	if req.Host != "example.test" {
		t.Errorf("Host = %q, want example.test", req.Host)
	}
}
