package benchmark

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xenon2333/netspeed/internal/engine"
)

const retryDelay = 200 * time.Millisecond

type statusError struct {
	url  string
	code int
}

func (e *statusError) Error() string {
	return fmt.Sprintf("%s 返回 HTTP %d", e.url, e.code)
}

func DownloadWorker(client *http.Client, cfg Config) engine.WorkerFunc {
	return func(ctx context.Context, counter *engine.TrafficCounter) error {
		buf := make([]byte, cfg.BufferSize)
		if len(buf) == 0 {
			buf = make([]byte, 32*1024)
		}
		var firstErr error
		for ctx.Err() == nil {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.Target, nil)
			if err != nil {
				return err
			}
			applyHeaders(req, cfg.Headers, cfg.UserAgent)

			resp, err := client.Do(req)
			if err != nil {
				if ctx.Err() != nil {
					break
				}
				if firstErr == nil {
					firstErr = err
				}
				if !waitRetry(ctx) {
					break
				}
				continue
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				resp.Body.Close()
				return &statusError{url: cfg.Target, code: resp.StatusCode}
			}

			readErr := readStream(counter, resp.Body, buf)
			resp.Body.Close()
			if readErr != nil {
				if ctx.Err() != nil {
					break
				}
				if firstErr == nil {
					firstErr = readErr
				}
				if !waitRetry(ctx) {
					break
				}
			}
		}
		if counter.Total() == 0 && firstErr != nil {
			return firstErr
		}
		return nil
	}
}

func readStream(counter *engine.TrafficCounter, body io.Reader, buf []byte) error {
	for {
		n, err := body.Read(buf)
		if n > 0 {
			counter.Add(int64(n))
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func waitRetry(ctx context.Context) bool {
	timer := time.NewTimer(retryDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
