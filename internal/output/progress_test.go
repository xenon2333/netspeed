package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xenon2333/netspeed/internal/engine"
)

func TestProgressLineMode(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, false)
	p.Report(engine.Sample{Elapsed: 2 * time.Second, InstantBps: 80e6, AvgBps: 100e6, TotalBytes: 25_000_000})
	p.Report(engine.Sample{Elapsed: 4 * time.Second, InstantBps: 90e6, AvgBps: 90e6, TotalBytes: 45_000_000})
	p.Finish()

	got := buf.String()
	if strings.Contains(got, "\r") {
		t.Errorf("line mode must not use carriage return: %q", got)
	}
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "瞬时 80.00 Mbit/s") || !strings.Contains(lines[0], "平均 100.00 Mbit/s") {
		t.Errorf("first line = %q", lines[0])
	}
	if !strings.Contains(lines[0], "已传输 23.84 MiB") {
		t.Errorf("first line missing transferred bytes: %q", lines[0])
	}
	if !strings.Contains(lines[1], "平均 90.00 Mbit/s") {
		t.Errorf("second line = %q", lines[1])
	}
}

func TestProgressInteractive(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.Report(engine.Sample{Elapsed: 2 * time.Second, InstantBps: 80e6, AvgBps: 100e6, TotalBytes: 25_000_000})
	p.Finish()

	got := buf.String()
	if !strings.HasPrefix(got, "\r") {
		t.Errorf("interactive output must start with carriage return: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("Finish must end the line: %q", got)
	}
	if strings.Count(got, "\r") != 1 {
		t.Errorf("want one refreshed line, got %q", got)
	}
}

func TestProgressFinishTwice(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, true)
	p.Report(engine.Sample{Elapsed: time.Second, InstantBps: 1e6, AvgBps: 1e6, TotalBytes: 125_000})
	p.Finish()
	first := buf.Len()
	p.Finish()
	if buf.Len() != first {
		t.Errorf("second Finish wrote output: %q", buf.String())
	}
}

func TestProgressFinishWithoutSamples(t *testing.T) {
	var buf bytes.Buffer
	NewProgress(&buf, true).Finish()
	NewProgress(&buf, false).Finish()
	if buf.Len() != 0 {
		t.Errorf("Finish without samples wrote %q", buf.String())
	}
}

func TestProgressAverageWithZeroElapsed(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgress(&buf, false)
	p.Report(engine.Sample{Elapsed: 0, InstantBps: 0, TotalBytes: 0})
	if !strings.Contains(buf.String(), "平均 0.00 bit/s") {
		t.Errorf("zero elapsed must not divide by zero: %q", buf.String())
	}
}
