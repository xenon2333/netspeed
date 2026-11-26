package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xenon2333/netspeed/internal/output"
	"github.com/xenon2333/netspeed/internal/version"
)

func runCLI(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRunHelp(t *testing.T) {
	code, stdout, stderr := runCLI("--help")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d", code, ExitOK)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "用法：") {
		t.Errorf("help output missing usage header:\n%s", stdout)
	}
	flags := []string{
		"--mode", "--target", "--method", "--duration", "--connections",
		"--buffer-size", "--interval", "--timeout", "--upload-size", "--user-agent",
		"--header", "--insecure", "--ipv6", "--no-proxy", "--bind",
		"--listen", "--quiet", "--json", "--help", "--version",
	}
	for _, flag := range flags {
		if !strings.Contains(stdout, flag) {
			t.Errorf("help output missing %s", flag)
		}
	}
}

func TestRunVersion(t *testing.T) {
	code, stdout, stderr := runCLI("--version")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d", code, ExitOK)
	}
	if want := "netspeed " + version.Version + "\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestRunMissingTarget(t *testing.T) {
	code, stdout, stderr := runCLI()
	if code != ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, ExitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "必须指定 --target") {
		t.Errorf("stderr = %q, want missing target hint", stderr)
	}
}

func TestRunModeIsCaseSensitive(t *testing.T) {
	code, _, stderr := runCLI("--mode", "Serve")
	if code != ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "工作模式") {
		t.Errorf("stderr = %q, want mode error", stderr)
	}
}

func TestRunMethodIsCaseSensitive(t *testing.T) {
	code, _, stderr := runCLI("--target", "https://example.com/x", "--method", "get")
	if code != ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "请求方法") {
		t.Errorf("stderr = %q, want method error", stderr)
	}
}

func TestRunRejectsPositionalArgument(t *testing.T) {
	code, _, stderr := runCLI("https://example.com/x")
	if code != ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "位置参数") {
		t.Errorf("stderr = %q, want positional argument error", stderr)
	}
}

func TestRunUnknownFlag(t *testing.T) {
	code, _, stderr := runCLI("--nope")
	if code != ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "-nope") {
		t.Errorf("stderr = %q, want unknown flag name", stderr)
	}
}

func TestRunDownloadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), 4096))
	}))
	defer srv.Close()

	code, stdout, stderr := runCLI("-t", srv.URL, "-d", "100ms", "-c", "2", "-q", "--json")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (stderr=%s)", code, ExitOK, stderr)
	}
	var report output.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if report.URL != srv.URL || report.Direction != "download" || report.Connections != 2 {
		t.Errorf("report = %+v", report)
	}
	if report.Bytes <= 0 || report.BitsPerSecond <= 0 || report.PeakBitsPerSecond <= 0 {
		t.Errorf("report has empty measurements: %+v", report)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty with -q", stderr)
	}
}

func TestRunTextOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), 1024))
	}))
	defer srv.Close()

	code, stdout, stderr := runCLI("-t", srv.URL, "-d", "60ms", "-q")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (stderr=%s)", code, ExitOK, stderr)
	}
	if !strings.Contains(stdout, "下载测速: "+srv.URL) {
		t.Errorf("stdout missing text report:\n%s", stdout)
	}
}

func TestRunUploadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	code, stdout, stderr := runCLI(
		"-t", srv.URL, "--method", "POST", "--upload-size", "2048",
		"-d", "100ms", "-q", "--json",
	)
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (stderr=%s)", code, ExitOK, stderr)
	}
	var report output.Report
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if report.Direction != "upload" || report.Bytes <= 0 {
		t.Errorf("report = %+v", report)
	}
}

func TestRunDownloadFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer srv.Close()

	code, stdout, stderr := runCLI("-t", srv.URL, "-d", "1s", "-q")
	if code != ExitFailure {
		t.Fatalf("exit code = %d, want %d", code, ExitFailure)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "404") {
		t.Errorf("stderr = %q, want HTTP 404", stderr)
	}
}

func TestRunServeRejectsInvalidListen(t *testing.T) {
	code, _, stderr := runCLI("--mode", "serve", "-l", "99999")
	if code != ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "端口") {
		t.Errorf("stderr = %q, want port range error", stderr)
	}
}

func TestRunInterruptedExitCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), 4096))
	}))
	defer srv.Close()

	o, err := parse([]string{"-t", srv.URL, "-d", "0", "-q", "--json"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := validate(o); err != nil {
		t.Fatalf("validate: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	var stdout, stderr bytes.Buffer
	code := runTestContext(ctx, o, &stdout, &stderr)
	if code != ExitInterrupted {
		t.Fatalf("exit code = %d, want %d (stderr=%s)", code, ExitInterrupted, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Error("interrupted run produced no report")
	}
}
