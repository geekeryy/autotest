package authprovider

import (
	"errors"
	"net/url"
	"strings"
)

var ErrFrontendOriginInvalid = errors.New("origin must be a valid URL with scheme and host")

// NormalizeFrontendOrigin trims whitespace, removes trailing slash, and validates scheme+host (no path).
func NormalizeFrontendOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrFrontendOriginInvalid
	}
	raw = strings.TrimRight(raw, "/")
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", ErrFrontendOriginInvalid
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrFrontendOriginInvalid
	}
	if parsed.Host == "" {
		return "", ErrFrontendOriginInvalid
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", ErrFrontendOriginInvalid
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrFrontendOriginInvalid
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func normalizeTrustedFrontendOrigins(raw []string) ([]string, error) {
	if raw == nil {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		normalized, err := NormalizeFrontendOrigin(item)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out, nil
}
