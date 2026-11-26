package benchmark

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	Target      string
	Method      string
	Headers     []string
	UserAgent   string
	Insecure    bool
	IPv6        bool
	NoProxy     bool
	Bind        string
	BufferSize  int64
	UploadSize  int64
	Timeout     time.Duration
	Connections int
}

func Direction(method string) string {
	if method == http.MethodGet {
		return "download"
	}
	return "upload"
}

func NewClient(cfg Config) (*http.Client, error) {
	dialer := &net.Dialer{Timeout: cfg.Timeout}
	if cfg.Bind != "" {
		ip := net.ParseIP(cfg.Bind)
		if ip == nil {
			return nil, fmt.Errorf("benchmark: 无效的绑定地址 %q", cfg.Bind)
		}
		dialer.LocalAddr = &net.TCPAddr{IP: ip}
	}
	network := "tcp"
	if cfg.IPv6 {
		network = "tcp6"
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		},
		ForceAttemptHTTP2:     false,
		TLSNextProto:          map[string]func(string, *tls.Conn) http.RoundTripper{},
		TLSHandshakeTimeout:   cfg.Timeout,
		ResponseHeaderTimeout: cfg.Timeout,
		DisableCompression:    true,
		MaxIdleConnsPerHost:   cfg.Connections,
		MaxConnsPerHost:       cfg.Connections,
		IdleConnTimeout:       90 * time.Second,
	}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	if cfg.NoProxy {
		transport.Proxy = nil
	}
	return &http.Client{Transport: transport}, nil
}

func applyHeaders(req *http.Request, headers []string, userAgent string) {
	req.Header.Set("User-Agent", userAgent)
	for _, raw := range headers {
		name, value, ok := strings.Cut(raw, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		switch {
		case strings.EqualFold(name, "Host"):
			req.Host = value
		case strings.EqualFold(name, "User-Agent"):
			req.Header.Set("User-Agent", value)
		default:
			req.Header.Add(name, value)
		}
	}
}
