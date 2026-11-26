# netspeed
基于 Go 标准库的命令行网络测速工具：对任意 HTTP/HTTPS 地址做持续下载或上传吞吐测试，也可以作为极简测速服务端使用。单二进制、无第三方依赖，响应体流式计数，不写磁盘、不整块读入内存。

## 特性
- 下载与上传测速：方向由 `--method` 决定，支持多连接并发。
- 实时进度与结果报告：进度写 stderr，结果写 stdout，可切换为 JSON。
- 自建服务端：`--mode serve` 提供 `/download`、`/upload` 端点，便于内网或离线环境互测。
- 可中断：Ctrl+C 后汇总已传输数据并退出，退出码 130。
- 跨平台：Windows、macOS、Linux，仅依赖 Go 标准库，使用 HTTP/1.1。

## 环境要求
- Go 1.23+（构建时）
- 无运行时依赖

## 安装
从源码构建：

```bash
git clone https://github.com/xenon2333/netspeed.git
cd netspeed
go build -o netspeed .
```

或直接安装：

```bash
go install github.com/xenon2333/netspeed@latest
```

## 快速开始
```bash
# 下载测速：4 条连接、持续 10 秒
netspeed -t "https://speed.cloudflare.com/__down?bytes=10000000" -d 10s -c 4

# 上传测速
netspeed -t "https://speed.cloudflare.com/__up" --method POST -d 10s -c 4

# 本机自建服务端，再从另一终端打流
netspeed --mode serve -l 127.0.0.1:9898
netspeed -t "http://127.0.0.1:9898/download" -d 10s -c 4
```

> [!Note]
> Cloudflare 的 `__down` 对单次请求的 `bytes` 上限为 100,000,000 字节（`bytes=99999999` 可用），请勿超过；需要更长测试时用 `-d` 延长，客户端会在每次响应结束后自动重新请求。

## 用法
```text
netspeed [选项]
```

### 测试模式（默认）
方向由 `--method` 决定：`GET` 为下载，`POST`/`PUT` 为上传。

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-t, --target <URL>` | — | 目标 URL，仅支持 `http`/`https` |
| `--method <GET\|POST\|PUT>` | `GET` | 请求方法，决定测试方向 |
| `-d, --duration <时长>` | `10s` | 测试持续时间，`0` 表示直到中断 |
| `-c, --connections <数量>` | `1` | 并发连接数 |
| `--buffer-size <大小>` | `64KiB` | 每条下载连接的读取缓冲区 |
| `--interval <时长>` | `500ms` | 采样间隔 |
| `--timeout <时长>` | `30s` | 等待响应头的超时，同时用于拨号与 TLS 握手 |
| `--upload-size <大小>` | `4MiB` | 每次上传请求的载荷大小 |
| `--user-agent <值>` | `netspeed/0.1.0` | User-Agent 请求头 |
| `--header <Name: value>` | — | 追加请求头，可重复；`Host`、`User-Agent` 单独处理 |
| `--insecure` | `false` | 跳过 TLS 证书校验 |
| `--ipv6` | `false` | 强制使用 IPv6 |
| `--no-proxy` | `false` | 忽略系统代理（`HTTP_PROXY` 等） |
| `--bind <IP>` | — | 绑定出口 IP，地址族需与 `--ipv6` 一致 |

时间格式支持 `300ms`、`10s`、`1m`；大小支持 `B`、`KB`、`MB`、`GB`（1000 进制）与 `KiB`、`MiB`、`GiB`（1024 进制），单位大小写不敏感，省略单位时按字节处理。

### 服务模式
| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `-m, --mode serve` | `test` | 启动测速服务端 |
| `-l, --listen <端口\|地址:端口>` | `9898` | 监听地址；只给端口时监听所有网卡 |

| 方法 | 路径 | 行为 |
| --- | --- | --- |
| GET | `/download` | 持续输出数据直到客户端断开 |
| GET | `/download?size=<字节数>` | 输出指定字节数后结束；`size` 非法返回 400 |
| POST / PUT | `/upload` | 丢弃请求体并返回 `{"bytes":<接收字节数>}` |
| GET | `/` | 端点说明 |

其余情况返回 405（附带 `Allow`）或 404。服务端默认监听所有网卡；本机自测建议显式绑定回环地址（`-l 127.0.0.1:9898`），避免触发系统防火墙提示。访问日志写 stderr，每请求一行（方法、路径、状态码、字节数、耗时）；Ctrl+C 触发优雅退出。

### 输出与通用选项
| 参数 | 说明 |
| --- | --- |
| `-q, --quiet` | 关闭实时进度 |
| `--json` | 以 JSON 格式输出结果 |
| `-h, --help` | 查看用法 |
| `-v, --version` | 查看版本号 |

枚举参数（`--mode`、`--method`）区分大小写。

## 输出
实时进度写 stderr：交互终端单行刷新，输出被重定向时每个采样单独一行。

```text
[  3.5s] 瞬时 98.42 Mbit/s   平均 95.10 Mbit/s   已传输 41.63 MiB
```

最终结果写 stdout：

```text
下载测速: https://example.com/test.bin
并发连接: 8      实际时长: 10.0s

平均速度: 95.37 Mbit/s
峰值速度: 102.11 Mbit/s
传输数据: 119.21 MiB
```

速率使用 SI 单位（kbit/s、Mbit/s、Gbit/s），字节量使用二进制单位（KiB、MiB、GiB）。

`--json` 时 stdout 只输出一个 JSON 对象：

```json
{
  "url": "https://example.com/test.bin",
  "direction": "download",
  "duration": 10.003,
  "bytes": 125000000,
  "bits_per_second": 99976007.2,
  "connections": 8,
  "peak_bits_per_second": 104857600
}
```

`--json` 只改变 stdout 格式，不关闭进度；需要完全安静时再加 `-q`。失败时不输出 JSON，错误只写 stderr。

## 退出码

| 退出码 | 含义 |
| --- | --- |
| `0` | 成功 |
| `1` | 测试失败：网络错误、非 2xx、没有任何数据等 |
| `2` | 参数错误 |
| `130` | 被中断且已有数据传输（结果照常输出） |

## 行为说明

- 非 2xx 响应视为终止性错误，立即失败、不重试。
- 连接重置、body 提前结束等瞬时错误每 200ms 重试；只要传过数据，按 `--duration` 结束即输出结果。
- 跟随重定向（最多 10 次）。上传遇到 307/308 会重放载荷；301/302/303 会被 Go 改为 GET 并丢弃请求体，因此上传目标应使用 307/308。
- 禁用 HTTP/2，`--connections` 等于真实 TCP 连接数；请求不声明压缩，避免影响测量。
- 下载忽略 `--upload-size`，上传忽略 `--buffer-size`。
- `--duration 0` 表示直到中断；目标完全不可达时会持续重试。

## 已知限制
- 有意不支持 HTTP/2 ；响应头到达后 body 停滞时没有独立读超时。
- 峰值是采样间隔内的平均速率，间隔越短越接近瞬时峰值。
- 服务端不限制并发与带宽，公网部署需自行控制暴露面。