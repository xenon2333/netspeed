package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/xenon2333/netspeed/internal/benchmark"
	"github.com/xenon2333/netspeed/internal/engine"
	"github.com/xenon2333/netspeed/internal/output"
	"github.com/xenon2333/netspeed/internal/server"
	"github.com/xenon2333/netspeed/internal/version"
)

const (
	ExitOK          = 0
	ExitFailure     = 1
	ExitUsage       = 2
	ExitInterrupted = 130
)

func Run(args []string, stdout, stderr io.Writer) int {
	o, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "netspeed: %v\n\n", err)
		printUsage(stderr)
		return ExitUsage
	}
	if o.help {
		printUsage(stdout)
		return ExitOK
	}
	if o.version {
		fmt.Fprintf(stdout, "netspeed %s\n", version.Version)
		return ExitOK
	}
	if err := validate(o); err != nil {
		fmt.Fprintf(stderr, "netspeed: %v\n\n", err)
		printUsage(stderr)
		return ExitUsage
	}
	if o.mode == serveMode {
		return runServe(o, stdout, stderr)
	}
	return runTest(o, stdout, stderr)
}

func runTest(o *options, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runTestContext(ctx, o, stdout, stderr)
}

func runTestContext(ctx context.Context, o *options, stdout, stderr io.Writer) int {
	cfg := benchmark.Config{
		Target:      o.target,
		Method:      o.method,
		Headers:     []string(o.headers),
		UserAgent:   o.userAgent,
		Insecure:    o.insecure,
		IPv6:        o.ipv6,
		NoProxy:     o.noProxy,
		Bind:        o.bind,
		BufferSize:  o.bufferBytes,
		UploadSize:  o.uploadBytes,
		Timeout:     o.timeout,
		Connections: o.connections,
	}
	client, err := benchmark.NewClient(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "netspeed: %v\n", err)
		return ExitFailure
	}

	var worker engine.WorkerFunc
	if o.method == http.MethodGet {
		worker = benchmark.DownloadWorker(client, cfg)
	} else {
		worker, err = benchmark.UploadWorker(client, cfg)
		if err != nil {
			fmt.Fprintf(stderr, "netspeed: %v\n", err)
			return ExitFailure
		}
	}

	runCfg := engine.Config{
		Duration: o.duration,
		Workers:  o.connections,
		Interval: o.interval,
	}
	var progress *output.Progress
	if !o.quiet {
		progress = output.NewProgress(stderr, isTerminal(stderr))
		runCfg.Reporter = progress
	}

	res, err := engine.Run(ctx, runCfg, worker)
	if progress != nil {
		progress.Finish()
	}
	if err != nil {
		if errors.Is(err, engine.ErrNoTraffic) {
			fmt.Fprintln(stderr, "netspeed: 没有传输任何数据")
		} else {
			fmt.Fprintf(stderr, "netspeed: %v\n", err)
		}
		return ExitFailure
	}

	report := output.Report{
		URL:               o.target,
		Direction:         benchmark.Direction(o.method),
		Duration:          res.Duration,
		Bytes:             res.Bytes,
		BitsPerSecond:     res.Bps,
		Connections:       res.Workers,
		PeakBitsPerSecond: res.PeakBps,
	}
	if o.jsonOut {
		if err := output.WriteJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "netspeed: %v\n", err)
			return ExitFailure
		}
	} else if err := output.WriteText(stdout, report); err != nil {
		fmt.Fprintf(stderr, "netspeed: %v\n", err)
		return ExitFailure
	}

	if ctx.Err() != nil {
		return ExitInterrupted
	}
	return ExitOK
}

func runServe(o *options, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(stderr, "netspeed: 监听 %s\n", o.listen)
	if err := server.Run(ctx, server.Config{Listen: o.listen, Logger: stderr}); err != nil {
		fmt.Fprintf(stderr, "netspeed: %v\n", err)
		return ExitFailure
	}
	return ExitOK
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
