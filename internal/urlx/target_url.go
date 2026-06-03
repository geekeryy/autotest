package urlx

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ErrInvalidTargetURL 表示目标 URL 不合法或不允许访问。
var ErrInvalidTargetURL = errors.New("target url is invalid")

// ValidateHTTPServiceURL 校验 HTTP(S) 服务 URL，阻止 SSRF（内网/本机/非 HTTP 协议等）。
func ValidateHTTPServiceURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("%w: url is required", ErrInvalidTargetURL)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTargetURL, err)
	}
	if u.User != nil {
		return fmt.Errorf("%w: userinfo is not allowed", ErrInvalidTargetURL)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: only http and https are allowed", ErrInvalidTargetURL)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("%w: host is required", ErrInvalidTargetURL)
	}

	if err := validateHost(u.Hostname()); err != nil {
		return err
	}
	return nil
}

func validateHost(host string) error {
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("%w: localhost is not allowed", ErrInvalidTargetURL)
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w: private or loopback address is not allowed", ErrInvalidTargetURL)
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve host: %v", ErrInvalidTargetURL, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: host resolved to no addresses", ErrInvalidTargetURL)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("%w: host resolves to private or loopback address", ErrInvalidTargetURL)
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	return ip.IsUnspecified()
}
