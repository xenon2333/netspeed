package cli

import (
	"fmt"
	"io"

	"github.com/xenon2333/netspeed/internal/version"
)

const usageTemplate = `用法： netspeed [选项]

工作模式：
  -m, --mode <serve|test>       工作模式，默认 test

测试模式选项：
  -t, --target <URL>            目标 URL，仅支持 http/https
      --method <GET|POST|PUT>   请求方法，默认 GET；POST/PUT 表示上传
  -d, --duration <时长>         测试持续时间，0 表示直到中断，默认 10s
  -c, --connections <数量>      并发连接数，默认 1
      --buffer-size <大小>      每条下载连接的缓冲区，默认 64KiB
      --interval <时长>         采样间隔，默认 500ms
      --timeout <时长>          等待响应头的超时，默认 30s
      --upload-size <大小>      每次上传请求的载荷，默认 4MiB
      --user-agent <值>         User-Agent 请求头，默认 netspeed/%s
      --header <Name: value>    追加请求头，可重复
      --insecure                跳过 TLS 证书校验
      --ipv6                    强制使用 IPv6
      --no-proxy                忽略系统代理
      --bind <IP>               绑定出口 IP

服务模式选项：
  -l, --listen <端口|地址:端口> 监听地址，默认 9898

输出选项：
  -q, --quiet                   关闭实时进度
      --json                    以 JSON 格式输出结果

其它：
  -h, --help                    查看用法
  -v, --version                 查看版本号

示例：
  netspeed --target https://example.com/test.bin --duration 10s --connections 8
  netspeed -t https://example.com/upload --method PUT -d 5s --json
  netspeed --mode serve -l 8080
`

func printUsage(w io.Writer) {
	fmt.Fprintf(w, usageTemplate, version.Version)
}
