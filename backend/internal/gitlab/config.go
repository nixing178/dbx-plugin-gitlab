package gitlab

import (
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParseBaseURL 归一化 GitLab 实例地址。
// 默认端口（https/443、http/80）会被省略，上下文路径自动补前导斜杠。
func ParseBaseURL(scheme, host string, port int, basePath string) (string, error) {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme == "" {
		return "", invalidConfig("protocol is required")
	}
	if scheme != "http" && scheme != "https" {
		return "", invalidConfig("unsupported protocol %q", scheme)
	}

	host = strings.TrimSpace(host)
	if host == "" {
		return "", invalidConfig("host is required")
	}
	if strings.Contains(host, "://") {
		return "", invalidConfig("host must not include the protocol")
	}
	if strings.ContainsAny(host, "/?#") {
		return "", invalidConfig("host must not include a path")
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "", invalidConfig("host must not include a port")
	}

	if port == 0 {
		if scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}
	if port < 1 || port > 65535 {
		return "", invalidConfig("port %d is out of range", port)
	}

	path := strings.Trim(strings.TrimSpace(basePath), "/")
	if strings.ContainsAny(path, " ?#") {
		return "", invalidConfig("context path must not contain spaces or query characters")
	}

	authority := host
	defaultPort := (scheme == "https" && port == 443) || (scheme == "http" && port == 80)
	if !defaultPort {
		authority = net.JoinHostPort(host, strconv.Itoa(port))
	}

	base := scheme + "://" + authority
	if path != "" {
		base += "/" + path
	}

	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" {
		return "", invalidConfig("invalid GitLab endpoint")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}
