package cli

import (
	"strings"
	"testing"
	"time"
)

func validTestOptions() *options {
	return &options{
		mode:        testMode,
		target:      "https://example.com/test.bin",
		method:      "GET",
		duration:    10 * time.Second,
		connections: 1,
		bufferSize:  "64KiB",
		interval:    500 * time.Millisecond,
		timeout:     30 * time.Second,
		uploadSize:  "4MiB",
		userAgent:   "netspeed/test",
		listen:      "9898",
	}
}

func TestValidateTestInvalid(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*options)
		want   string
	}{
		{"missing target", func(o *options) { o.target = "" }, "必须指定"},
		{"invalid url", func(o *options) { o.target = "://bad" }, "无效的目标 URL"},
		{"unsupported scheme", func(o *options) { o.target = "ftp://example.com/x" }, "不支持的协议"},
		{"missing host", func(o *options) { o.target = "https:///x" }, "缺少主机名"},
		{"lowercase method", func(o *options) { o.method = "get" }, "请求方法"},
		{"negative duration", func(o *options) { o.duration = -time.Second }, "--duration"},
		{"zero timeout", func(o *options) { o.timeout = 0 }, "--timeout"},
		{"zero connections", func(o *options) { o.connections = 0 }, "--connections"},
		{"zero interval", func(o *options) { o.interval = 0 }, "--interval"},
		{"invalid buffer size", func(o *options) { o.bufferSize = "abc" }, "--buffer-size"},
		{"zero buffer size", func(o *options) { o.bufferSize = "0" }, "--buffer-size"},
		{"invalid upload size", func(o *options) { o.uploadSize = "10XB" }, "--upload-size"},
		{"invalid bind", func(o *options) { o.bind = "not-an-ip" }, "--bind"},
		{"ipv4 bind with ipv6", func(o *options) { o.ipv6 = true; o.bind = "192.168.1.10" }, "IPv4"},
		{"ipv6 bind without flag", func(o *options) { o.bind = "::1" }, "--ipv6"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := validTestOptions()
			c.mutate(o)
			err := validate(o)
			if err == nil {
				t.Fatalf("validate succeeded, want error containing %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %q, want substring %q", err, c.want)
			}
		})
	}
}

func TestValidateTestValid(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*options)
	}{
		{"defaults", func(*options) {}},
		{"post upload", func(o *options) { o.method = "POST" }},
		{"put upload", func(o *options) { o.method = "PUT"; o.duration = 0 }},
		{"ipv4 bind", func(o *options) { o.bind = "192.168.1.10" }},
		{"ipv6 bind", func(o *options) { o.ipv6 = true; o.bind = "::1" }},
		{"positional sizes", func(o *options) { o.bufferSize = "128KiB"; o.uploadSize = "8MiB" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := validTestOptions()
			c.mutate(o)
			if err := validate(o); err != nil {
				t.Fatalf("validate: %v", err)
			}
		})
	}
}

func TestValidateNormalizesScheme(t *testing.T) {
	o := validTestOptions()
	o.target = "HTTPS://Example.com/x"
	if err := validate(o); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !strings.HasPrefix(o.target, "https://") {
		t.Errorf("target = %q, want https prefix", o.target)
	}
}

func TestValidateServeListen(t *testing.T) {
	cases := []struct {
		listen  string
		wantErr bool
	}{
		{"9898", false},
		{"8080", false},
		{"127.0.0.1:8080", false},
		{":8080", false},
		{"[::1]:8080", false},
		{"0", true},
		{"65536", true},
		{"abc", true},
		{"127.0.0.1", true},
		{"", true},
	}
	for _, c := range cases {
		t.Run(c.listen, func(t *testing.T) {
			err := validate(&options{mode: serveMode, listen: c.listen})
			if (err != nil) != c.wantErr {
				t.Fatalf("validate(serve, listen=%q) error = %v, wantErr = %v", c.listen, err, c.wantErr)
			}
		})
	}
}

func TestValidateServeIgnoresTestFlags(t *testing.T) {
	o := validTestOptions()
	o.mode = serveMode
	o.target = ""
	o.method = "get"
	o.connections = 0
	o.bufferSize = "nonsense"
	if err := validate(o); err != nil {
		t.Fatalf("serve mode should ignore test flags, got %v", err)
	}
}
