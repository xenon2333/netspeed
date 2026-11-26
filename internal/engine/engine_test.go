package engine

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func streamWorker(bytesPerTick int64) WorkerFunc {
	return func(ctx context.Context, c *TrafficCounter) error {
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				c.Add(bytesPerTick)
			}
		}
	}
}

type sampleRecorder struct {
	mu      sync.Mutex
	samples []Sample
}

func (r *sampleRecorder) Report(s Sample) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.samples = append(r.samples, s)
}

func (r *sampleRecorder) snapshot() []Sample {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Sample(nil), r.samples...)
}

func TestRunReportsSamples(t *testing.T) {
	recorder := &sampleRecorder{}
	cfg := Config{
		Duration: 200 * time.Millisecond,
		Workers:  2,
		Interval: 10 * time.Millisecond,
		Reporter: recorder,
	}
	res, err := Run(context.Background(), cfg, streamWorker(1000))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Bytes <= 0 || res.Workers != 2 {
		t.Errorf("result = %+v, want bytes > 0 and 2 workers", res)
	}

	samples := recorder.snapshot()
	if len(samples) == 0 {
		t.Fatal("reporter received no samples")
	}
	for _, s := range samples {
		if s.Elapsed <= 0 {
			t.Errorf("sample elapsed = %v, want > 0", s.Elapsed)
		}
		wantAvg := float64(s.TotalBytes) * 8 / s.Elapsed.Seconds()
		if math.Abs(s.AvgBps-wantAvg) > 1e-6*wantAvg {
			t.Errorf("AvgBps = %v, want %v", s.AvgBps, wantAvg)
		}
	}
}

func TestRunStopsAtDuration(t *testing.T) {
	res, err := Run(context.Background(), Config{Duration: 80 * time.Millisecond}, streamWorker(1000))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Duration < 0.08 || res.Duration > 0.5 {
		t.Errorf("Duration = %v, want about 0.08s", res.Duration)
	}
	if res.Bytes <= 0 {
		t.Errorf("Bytes = %d, want > 0", res.Bytes)
	}
}

func TestRunContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	worker := func(ctx context.Context, c *TrafficCounter) error {
		c.Add(1000)
		<-ctx.Done()
		return nil
	}
	res, err := Run(ctx, Config{}, worker)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Bytes < 1000 {
		t.Errorf("Bytes = %d, want >= 1000", res.Bytes)
	}
}

func TestRunReturnsWorkerError(t *testing.T) {
	boom := errors.New("boom")
	worker := func(context.Context, *TrafficCounter) error { return boom }
	res, err := Run(context.Background(), Config{}, worker)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if res != nil {
		t.Errorf("result = %+v, want nil", res)
	}
}

func TestRunWorkerErrorCancelsOthers(t *testing.T) {
	boom := errors.New("boom")
	var calls atomic.Int32
	worker := func(ctx context.Context, c *TrafficCounter) error {
		if calls.Add(1) == 1 {
			return boom
		}
		<-ctx.Done()
		return nil
	}

	done := make(chan error, 1)
	go func() {
		_, err := Run(context.Background(), Config{Workers: 2}, worker)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want boom", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not cancel other workers")
	}
}

func TestRunNoTraffic(t *testing.T) {
	worker := func(context.Context, *TrafficCounter) error { return nil }
	if _, err := Run(context.Background(), Config{}, worker); !errors.Is(err, ErrNoTraffic) {
		t.Fatalf("err = %v, want ErrNoTraffic", err)
	}
}

func TestRunNilWorker(t *testing.T) {
	if _, err := Run(context.Background(), Config{}, nil); err == nil {
		t.Fatal("Run(nil worker) succeeded, want error")
	}
}

func TestRunPeakFallsBackToAverage(t *testing.T) {
	worker := func(ctx context.Context, c *TrafficCounter) error {
		c.Add(1000)
		<-ctx.Done()
		return nil
	}
	res, err := Run(context.Background(), Config{Duration: 20 * time.Millisecond, Interval: time.Hour}, worker)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.PeakBps != res.Bps {
		t.Errorf("PeakBps = %v, want fallback to Bps %v", res.PeakBps, res.Bps)
	}
	if res.PeakBps <= 0 {
		t.Errorf("PeakBps = %v, want > 0", res.PeakBps)
	}
}

func TestRunNormalizesWorkers(t *testing.T) {
	res, err := Run(context.Background(), Config{Duration: 30 * time.Millisecond, Workers: 0}, streamWorker(1000))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Workers != DefaultWorkers {
		t.Errorf("Workers = %d, want %d", res.Workers, DefaultWorkers)
	}
}
