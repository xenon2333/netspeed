package cli

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func validate(o *options) error {
	switch o.mode {
	case testMode:
		return validateTest(o)
	case serveMode:
		return validateServe(o)
	default:
		return fmt.Errorf("未知的工作模式 %q，可选 serve 或 test", o.mode)
	}
}

func validateTest(o *options) error {
	if o.target == "" {
		return fmt.Errorf("必须指定 --target")
	}
	u, err := url.Parse(o.target)
	if err != nil {
		return fmt.Errorf("无效的目标 URL %q：%v", o.target, err)
	}
	if u.Host == "" {
		return fmt.Errorf("无效的目标 URL %q：缺少主机名", o.target)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("不支持的协议 %q，仅支持 http 或 https", u.Scheme)
	}
	u.Scheme = scheme
	o.target = u.String()

	switch o.method {
	case "GET", "POST", "PUT":
	default:
		return fmt.Errorf("不支持的请求方法 %q，可选 GET、POST、PUT", o.method)
	}

	if o.duration < 0 {
		return fmt.Errorf("--duration 不能为负数")
	}
	if o.timeout <= 0 {
		return fmt.Errorf("--timeout 必须大于 0")
	}
	if o.connections < 1 {
		return fmt.Errorf("--connections 必须大于 0")
	}
	if o.interval <= 0 {
		return fmt.Errorf("--interval 必须大于 0")
	}

	size, err := parseSize(o.bufferSize)
	if err != nil {
		return fmt.Errorf("--buffer-size：%v", err)
	}
	o.bufferBytes = size

	size, err = parseSize(o.uploadSize)
	if err != nil {
		return fmt.Errorf("--upload-size：%v", err)
	}
	o.uploadBytes = size

	if o.bind != "" {
		ip := net.ParseIP(o.bind)
		if ip == nil {
			return fmt.Errorf("--bind 不是合法的 IP 地址：%q", o.bind)
		}
		if o.ipv6 && ip.To4() != nil {
			return fmt.Errorf("--ipv6 不能与 IPv4 的 --bind %s 同时使用", o.bind)
		}
		if !o.ipv6 && ip.To4() == nil {
			return fmt.Errorf("--bind %s 是 IPv6 地址，需要同时指定 --ipv6", o.bind)
		}
	}
	return nil
}

func validateServe(o *options) error {
	return validateListen(o.listen)
}

func validateListen(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("--listen 不能为空")
	}
	if port, err := strconv.Atoi(s); err == nil {
		return checkPort(port)
	}
	_, portText, err := net.SplitHostPort(s)
	if err != nil {
		return fmt.Errorf("无效的监听地址 %q，应为端口或 地址:端口", s)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return fmt.Errorf("无效的监听地址 %q：端口不是数字", s)
	}
	return checkPort(port)
}

func checkPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("端口 %d 超出范围 1-65535", port)
	}
	return nil
}

func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("大小不能为空")
	}
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	numPart := s[:i]
	if numPart == "" {
		return 0, fmt.Errorf("无效的大小 %q", s)
	}
	num, err := strconv.ParseFloat(numPart, 64)
	if err != nil || math.IsNaN(num) || math.IsInf(num, 0) {
		return 0, fmt.Errorf("无效的大小 %q", s)
	}

	unit := strings.ToUpper(strings.TrimSpace(s[i:]))
	var multiplier float64
	switch unit {
	case "", "B":
		multiplier = 1
	case "KB":
		multiplier = 1e3
	case "MB":
		multiplier = 1e6
	case "GB":
		multiplier = 1e9
	case "KIB":
		multiplier = 1 << 10
	case "MIB":
		multiplier = 1 << 20
	case "GIB":
		multiplier = 1 << 30
	default:
		return 0, fmt.Errorf("无效的大小 %q：未知单位 %q", s, strings.TrimSpace(s[i:]))
	}

	size := num * multiplier
	if size <= 0 {
		return 0, fmt.Errorf("无效的大小 %q：必须为正数", s)
	}
	if size >= float64(math.MaxInt64) {
		return 0, fmt.Errorf("无效的大小 %q：超出范围", s)
	}
	return int64(size), nil
}
