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

// OriginFromURL extracts scheme://host from an absolute URL (ignores path/query/fragment).
func OriginFromURL(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	return parsed.Scheme + "://" + parsed.Host, true
}

// CallbackURLMatchesOrigin reports whether callbackURL belongs to the given API origin.
func CallbackURLMatchesOrigin(callbackURL, apiOrigin string) bool {
	callbackOrigin, ok := OriginFromURL(callbackURL)
	if !ok {
		return false
	}
	apiOrigin = strings.TrimSpace(apiOrigin)
	if apiOrigin == "" {
		return false
	}
	return strings.EqualFold(callbackOrigin, apiOrigin)
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
