package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/xenon2333/netspeed/internal/version"
)

const (
	testMode  = "test"
	serveMode = "serve"
)

type options struct {
	mode        string
	target      string
	method      string
	duration    time.Duration
	connections int
	quiet       bool
	bufferSize  string
	interval    time.Duration
	timeout     time.Duration
	jsonOut     bool
	uploadSize  string
	userAgent   string
	insecure    bool
	headers     headerList
	ipv6        bool
	noProxy     bool
	bind        string
	listen      string
	help        bool
	version     bool

	bufferBytes int64
	uploadBytes int64
}

type headerList []string

func (h *headerList) String() string { return strings.Join(*h, ", ") }

func (h *headerList) Set(v string) error {
	name, _, ok := strings.Cut(v, ":")
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf(`请求头格式应为 "Name: value"`)
	}
	*h = append(*h, v)
	return nil
}

// parse 负责取值；帮助文本统一由 usage.go 维护，因此 flag 自带的描述留空。
func parse(args []string) (*options, error) {
	o := &options{}
	fs := flag.NewFlagSet("netspeed", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	fs.StringVar(&o.mode, "m", testMode, "")
	fs.StringVar(&o.mode, "mode", testMode, "")
	fs.StringVar(&o.target, "t", "", "")
	fs.StringVar(&o.target, "target", "", "")
	fs.StringVar(&o.method, "method", "GET", "")
	fs.DurationVar(&o.duration, "d", 10*time.Second, "")
	fs.DurationVar(&o.duration, "duration", 10*time.Second, "")
	fs.IntVar(&o.connections, "c", 1, "")
	fs.IntVar(&o.connections, "connections", 1, "")
	fs.BoolVar(&o.quiet, "q", false, "")
	fs.BoolVar(&o.quiet, "quiet", false, "")
	fs.StringVar(&o.bufferSize, "buffer-size", "64KiB", "")
	fs.DurationVar(&o.interval, "interval", 500*time.Millisecond, "")
	fs.DurationVar(&o.timeout, "timeout", 30*time.Second, "")
	fs.BoolVar(&o.jsonOut, "json", false, "")
	fs.StringVar(&o.uploadSize, "upload-size", "4MiB", "")
	fs.StringVar(&o.userAgent, "user-agent", "netspeed/"+version.Version, "")
	fs.BoolVar(&o.insecure, "insecure", false, "")
	fs.Var(&o.headers, "header", "")
	fs.BoolVar(&o.ipv6, "ipv6", false, "")
	fs.BoolVar(&o.noProxy, "no-proxy", false, "")
	fs.StringVar(&o.bind, "bind", "", "")
	fs.StringVar(&o.listen, "l", "9898", "")
	fs.StringVar(&o.listen, "listen", "9898", "")
	fs.BoolVar(&o.help, "h", false, "")
	fs.BoolVar(&o.help, "help", false, "")
	fs.BoolVar(&o.version, "v", false, "")
	fs.BoolVar(&o.version, "version", false, "")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 && !o.help && !o.version {
		return nil, fmt.Errorf("不支持位置参数 %q，请使用 --target", fs.Arg(0))
	}
	return o, nil
}
