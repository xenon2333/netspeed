package engine

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	DefaultWorkers  = 1
	DefaultInterval = 500 * time.Millisecond
)

var ErrNoTraffic = errors.New("no data was transferred")

type Config struct {
	Duration time.Duration // 0 表示直到 ctx 取消
	Workers  int           // 并发 worker 数
	Interval time.Duration // 采样间隔
	Reporter Reporter
}

type Reporter interface {
	Report(Sample)
}

type Sample struct {
	Elapsed    time.Duration // 自测试开始以来的时间
	InstantBps float64       // 最近一个采样间隔内观测到的速率
	AvgBps     float64       // 自测试开始以来的平均速率
	TotalBytes int64         // 累计传输字节数
}

type Result struct {
	Duration float64 // 实际耗时，秒
	Bytes    int64
	Bps      float64
	PeakBps  float64
	Workers  int
}

type WorkerFunc func(context.Context, *TrafficCounter) error

func Run(ctx context.Context, cfg Config, worker WorkerFunc) (*Result, error) {
	if worker == nil {
		return nil, errors.New("engine: worker must not be nil")
	}
	if cfg.Duration < 0 {
		cfg.Duration = 0
	}
	if cfg.Workers < 1 {
		cfg.Workers = DefaultWorkers
	}
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultInterval
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if cfg.Duration > 0 {
		var stop context.CancelFunc
		ctx, stop = context.WithTimeout(ctx, cfg.Duration)
		defer stop()
	}

	counter := &TrafficCounter{}
	start := time.Now()
	workerErrs := make(chan error, cfg.Workers)
	var wg sync.WaitGroup
	for range cfg.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := worker(ctx, counter); err != nil {
				select {
				case workerErrs <- err:
				default:
				}
				cancel()
			}
		}()
	}

	samplerDone := make(chan struct{})
	go func() {
		defer close(samplerDone)
		ticker := time.NewTicker(cfg.Interval)
		defer ticker.Stop()
		prevBytes := int64(0)
		prevTime := start
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				total := counter.Total()
				dt := now.Sub(prevTime).Seconds()
				if dt <= 0 {
					continue
				}
				instant := float64(total-prevBytes) * 8 / dt
				prevBytes, prevTime = total, now
				counter.UpdatePeak(instant)
				if cfg.Reporter != nil {
					elapsed := now.Sub(start)
					cfg.Reporter.Report(Sample{
						Elapsed:    elapsed,
						InstantBps: instant,
						AvgBps:     float64(total) * 8 / elapsed.Seconds(),
						TotalBytes: total,
					})
				}
			}
		}
	}()

	wg.Wait()
	cancel()
	<-samplerDone

	var firstErr error
	select {
	case firstErr = <-workerErrs:
	default:
	}
	if firstErr != nil {
		return nil, firstErr
	}

	elapsed := time.Since(start)
	if elapsed <= 0 {
		elapsed = time.Nanosecond
	}
	totalBytes := counter.Total()
	if totalBytes == 0 {
		return nil, ErrNoTraffic
	}
	bps := float64(totalBytes) * 8 / elapsed.Seconds()
	peakBps := counter.Peak()
	if peakBps <= 0 {
		peakBps = bps
	}
	return &Result{
		Duration: elapsed.Seconds(),
		Bytes:    totalBytes,
		Bps:      bps,
		PeakBps:  peakBps,
		Workers:  cfg.Workers,
	}, nil
}
