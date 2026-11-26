package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func testReport() Report {
	return Report{
		URL:               "https://example.com/test.bin",
		Direction:         "download",
		Duration:          10.003,
		Bytes:             125_000_000,
		BitsPerSecond:     99_976_007.2,
		Connections:       8,
		PeakBitsPerSecond: 104_857_600,
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, testReport()); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("JSON output does not end with newline: %q", buf.String())
	}

	var decoded Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded != testReport() {
		t.Errorf("round trip = %+v, want %+v", decoded, testReport())
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &fields); err != nil {
		t.Fatalf("unmarshal fields: %v", err)
	}
	wantFields := []string{
		"url", "direction", "duration", "bytes",
		"bits_per_second", "connections", "peak_bits_per_second",
	}
	if len(fields) != len(wantFields) {
		t.Errorf("JSON has %d fields, want %d: %v", len(fields), len(wantFields), fields)
	}
	for _, name := range wantFields {
		if _, ok := fields[name]; !ok {
			t.Errorf("JSON missing field %q", name)
		}
	}
	if !strings.Contains(buf.String(), "\n  \"url\"") {
		t.Errorf("JSON is not indented:\n%s", buf.String())
	}
}

func TestWriteText(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteText(&buf, testReport()); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	got := buf.String()
	wants := []string{
		"下载测速: https://example.com/test.bin",
		"并发连接: 8",
		"实际时长: 10.0s",
		"平均速度: 99.98 Mbit/s",
		"峰值速度: 104.86 Mbit/s",
		"传输数据: 119.21 MiB",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("text output missing %q:\n%s", want, got)
		}
	}
}

func TestWriteTextDirection(t *testing.T) {
	r := testReport()
	r.Direction = "upload"
	var buf bytes.Buffer
	if err := WriteText(&buf, r); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if !strings.Contains(buf.String(), "上传测速:") {
		t.Errorf("upload output missing label:\n%s", buf.String())
	}

	r.Direction = "unknown"
	buf.Reset()
	if err := WriteText(&buf, r); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if !strings.Contains(buf.String(), "测速:") {
		t.Errorf("fallback output missing label:\n%s", buf.String())
	}
}
