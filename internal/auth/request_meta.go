package auth

import (
	"net"
	"net/http"
	"strings"
)

// RequestMetaFromHTTP 从 HTTP 请求提取客户端 IP 与 User-Agent。
func RequestMetaFromHTTP(r *http.Request) RequestMeta {
	return RequestMeta{
		ClientIP:  clientIPFromRequest(r),
		UserAgent: strings.TrimSpace(r.UserAgent()),
	}
}

// RequestAPIOrigin returns the public API origin (scheme + host) for the incoming request.
func RequestAPIOrigin(r *http.Request) string {
	if r == nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		if v := strings.TrimSpace(strings.Split(proto, ",")[0]); v != "" {
			scheme = v
		}
	}
	host := strings.TrimSpace(r.Host)
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		host = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

func clientIPFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
