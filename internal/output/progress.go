package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/xenon2333/netspeed/internal/engine"
)

// Progress 渲染周期采样。interactive 为 true 时用 \r 覆盖同一行，
// 为 false 时每个采样单独一行，便于输出被重定向到文件。
type Progress struct {
	w           io.Writer
	interactive bool
	lastLen     int
}

func NewProgress(w io.Writer, interactive bool) *Progress {
	return &Progress{w: w, interactive: interactive}
}

func (p *Progress) Report(s engine.Sample) {
	line := fmt.Sprintf("[%5.1fs] 瞬时 %s   平均 %s   已传输 %s",
		s.Elapsed.Seconds(),
		FormatBitsPerSecond(s.InstantBps),
		FormatBitsPerSecond(s.AvgBps),
		FormatBytes(s.TotalBytes),
	)
	if !p.interactive {
		fmt.Fprintln(p.w, line)
		return
	}
	padding := 0
	if len(line) < p.lastLen {
		padding = p.lastLen - len(line)
	}
	fmt.Fprintf(p.w, "\r%s%s", line, strings.Repeat(" ", padding))
	p.lastLen = len(line)
}

// Finish 结束当前进度行；只有交互模式且输出过采样时才补换行。
func (p *Progress) Finish() {
	if p.interactive && p.lastLen > 0 {
		fmt.Fprintln(p.w)
		p.lastLen = 0
	}
}
