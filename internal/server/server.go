package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

var shutdownTimeout = 5 * time.Second

type Config struct {
	Listen string
	Logger io.Writer
}

func Run(ctx context.Context, cfg Config) error {
	ln, err := net.Listen("tcp", listenAddress(cfg.Listen))
	if err != nil {
		return err
	}
	return serve(ctx, ln, cfg.Logger)
}

func listenAddress(s string) string {
	if _, err := strconv.Atoi(s); err == nil {
		return ":" + s
	}
	return s
}

func serve(ctx context.Context, ln net.Listener, logger io.Writer) error {
	srv := &http.Server{
		Handler:           Handler(logger),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			// 仍有传输未结束时强制关闭，避免中断被长连接拖住。
			_ = srv.Close()
		}
		return nil
	}
}

func Handler(logger io.Writer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc("/download", downloadHandler)
	mux.HandleFunc("/upload", uploadHandler)
	return withAccessLog(logger, mux)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, `netspeed 测速服务

GET  /download?size=<字节数>   下载测速；省略 size 时持续输出直到客户端断开
POST /upload                   上传测速；返回 {"bytes": <接收字节数>}
PUT  /upload                   同上
GET  /                         本说明
`)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	remaining := int64(-1)
	if text := r.URL.Query().Get("size"); text != "" {
		size, err := strconv.ParseInt(text, 10, 64)
		if err != nil || size < 0 {
			http.Error(w, "invalid size", http.StatusBadRequest)
			return
		}
		remaining = size
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")

	block := randomBlock()
	for {
		select {
		case <-r.Context().Done():
			return
		default:
		}
		if remaining == 0 {
			return
		}
		chunk := block
		if remaining >= 0 && int64(len(chunk)) > remaining {
			chunk = chunk[:remaining]
		}
		n, err := w.Write(chunk)
		if err != nil {
			return
		}
		if remaining >= 0 {
			remaining -= int64(n)
		}
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		methodNotAllowed(w, http.MethodPost+", "+http.MethodPut)
		return
	}
	n, err := io.Copy(io.Discard, r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "{\"bytes\":%d}\n", n)
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func randomBlock() []byte {
	const blockSize = 64 << 10
	block := make([]byte, blockSize)
	var x uint64 = 0x9E3779B97F4A7C15
	for i := range block {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		block[i] = byte(x)
	}
	return block
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *loggingResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err
}

func withAccessLog(logger io.Writer, next http.Handler) http.Handler {
	if logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lw, r)
		if lw.status == 0 {
			lw.status = http.StatusOK
		}
		fmt.Fprintf(logger, "%s %s %d %dB %s\n",
			r.Method, r.URL.RequestURI(), lw.status, lw.bytes,
			time.Since(start).Round(time.Millisecond))
	})
}
