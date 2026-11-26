package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// Report 是最终结果的 JSON 契约，字段名与 README、设计文档保持一致。
type Report struct {
	URL               string  `json:"url"`
	Direction         string  `json:"direction"`
	Duration          float64 `json:"duration"`
	Bytes             int64   `json:"bytes"`
	BitsPerSecond     float64 `json:"bits_per_second"`
	Connections       int     `json:"connections"`
	PeakBitsPerSecond float64 `json:"peak_bits_per_second"`
}

func WriteJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func WriteText(w io.Writer, r Report) error {
	label := "测速"
	switch r.Direction {
	case "download":
		label = "下载测速"
	case "upload":
		label = "上传测速"
	}
	_, err := fmt.Fprintf(w, `%s: %s
并发连接: %d      实际时长: %.1fs

平均速度: %s
峰值速度: %s
传输数据: %s
`,
		label, r.URL, r.Connections, r.Duration,
		FormatBitsPerSecond(r.BitsPerSecond),
		FormatBitsPerSecond(r.PeakBitsPerSecond),
		FormatBytes(r.Bytes),
	)
	return err
}
