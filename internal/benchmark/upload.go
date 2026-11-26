package benchmark

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"net/http"

	"github.com/xenon2333/netspeed/internal/engine"
)

func UploadWorker(client *http.Client, cfg Config) (engine.WorkerFunc, error) {
	payload := make([]byte, cfg.UploadSize)
	if _, err := rand.Read(payload); err != nil {
		return nil, err
	}
	newBody := func(counter *engine.TrafficCounter) io.ReadCloser {
		return io.NopCloser(&countingReader{r: bytes.NewReader(payload), counter: counter})
	}
	return func(ctx context.Context, counter *engine.TrafficCounter) error {
		var firstErr error
		for ctx.Err() == nil {
			body := newBody(counter)
			req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.Target, body)
			if err != nil {
				return err
			}
			req.ContentLength = int64(len(payload))
			// 307/308 需要重放请求体；缺少 GetBody 时 net/http 会把 3xx 直接返回给调用方。
			req.GetBody = func() (io.ReadCloser, error) { return newBody(counter), nil }
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

			_, readErr := io.Copy(io.Discard, resp.Body)
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
	}, nil
}

type countingReader struct {
	r       io.Reader
	counter *engine.TrafficCounter
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	if n > 0 {
		cr.counter.Add(int64(n))
	}
	return n, err
}
