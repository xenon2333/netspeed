package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/xenon2333/netspeed/internal/version"
)

func TestParseDefaults(t *testing.T) {
	o, err := parse(nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if o.mode != testMode {
		t.Errorf("mode = %q, want %q", o.mode, testMode)
	}
	if o.target != "" {
		t.Errorf("target = %q, want empty", o.target)
	}
	if o.method != "GET" {
		t.Errorf("method = %q, want GET", o.method)
	}
	if o.duration != 10*time.Second {
		t.Errorf("duration = %v, want 10s", o.duration)
	}
	if o.connections != 1 {
		t.Errorf("connections = %d, want 1", o.connections)
	}
	if o.bufferSize != "64KiB" {
		t.Errorf("bufferSize = %q, want 64KiB", o.bufferSize)
	}
	if o.interval != 500*time.Millisecond {
		t.Errorf("interval = %v, want 500ms", o.interval)
	}
	if o.timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", o.timeout)
	}
	if o.uploadSize != "4MiB" {
		t.Errorf("uploadSize = %q, want 4MiB", o.uploadSize)
	}
	if want := "netspeed/" + version.Version; o.userAgent != want {
		t.Errorf("userAgent = %q, want %q", o.userAgent, want)
	}
	if o.listen != "9898" {
		t.Errorf("listen = %q, want 9898", o.listen)
	}
	if o.quiet || o.jsonOut || o.insecure || o.ipv6 || o.noProxy {
		t.Errorf("boolean flags should default to false: %+v", o)
	}
	if len(o.headers) != 0 {
		t.Errorf("headers = %v, want empty", o.headers)
	}
}

func TestParseFlags(t *testing.T) {
	o, err := parse([]string{
		"--mode", "serve", "--target", "http://example.com/x", "--method", "PUT",
		"-d", "5s", "-c", "8", "--buffer-size", "1MiB", "--interval", "250ms",
		"--timeout", "5s", "--json", "-q", "--upload-size", "2MiB",
		"--user-agent", "ua", "--insecure", "--header", "A: 1", "--header", "B: 2",
		"--ipv6", "--no-proxy", "--bind", "::1", "-l", "127.0.0.1:9999",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if o.mode != serveMode || o.target != "http://example.com/x" || o.method != "PUT" {
		t.Errorf("mode/target/method = %q/%q/%q", o.mode, o.target, o.method)
	}
	if o.duration != 5*time.Second || o.connections != 8 || o.interval != 250*time.Millisecond || o.timeout != 5*time.Second {
		t.Errorf("durations/counts not parsed: %+v", o)
	}
	if o.bufferSize != "1MiB" || o.uploadSize != "2MiB" || o.userAgent != "ua" {
		t.Errorf("sizes/userAgent not parsed: %+v", o)
	}
	if !o.jsonOut || !o.quiet || !o.insecure || !o.ipv6 || !o.noProxy {
		t.Errorf("boolean flags not parsed: %+v", o)
	}
	if len(o.headers) != 2 || o.headers[0] != "A: 1" || o.headers[1] != "B: 2" {
		t.Errorf("headers = %v", o.headers)
	}
	if o.bind != "::1" || o.listen != "127.0.0.1:9999" {
		t.Errorf("bind/listen = %q/%q", o.bind, o.listen)
	}
}

func TestParseShortFlags(t *testing.T) {
	o, err := parse([]string{"-m", "test", "-t", "http://example.com/x", "-d", "1s", "-c", "2", "-q", "-l", "8080", "-v"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if o.mode != testMode || o.target != "http://example.com/x" {
		t.Errorf("mode/target = %q/%q", o.mode, o.target)
	}
	if o.duration != time.Second || o.connections != 2 || !o.quiet || o.listen != "8080" || !o.version {
		t.Errorf("short flags not parsed: %+v", o)
	}
}

func TestParseRejectsLeftoverArgument(t *testing.T) {
	if _, err := parse([]string{"https://example.com/x"}); err == nil || !strings.Contains(err.Error(), "位置参数") {
		t.Fatalf("err = %v, want positional argument error", err)
	}
}

func TestParseHelpIgnoresLeftoverArgument(t *testing.T) {
	o, err := parse([]string{"--help", "https://example.com/x"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !o.help {
		t.Errorf("help = false, want true")
	}
}

func TestParseHeaderErrors(t *testing.T) {
	for _, value := range []string{"nocolon", ": value", ""} {
		if _, err := parse([]string{"--header", value}); err == nil {
			t.Errorf("parse(--header %q) succeeded, want error", value)
		}
	}
}

func TestParseUnknownFlag(t *testing.T) {
	if _, err := parse([]string{"--nope"}); err == nil {
		t.Fatal("parse(--nope) succeeded, want error")
	}
}
