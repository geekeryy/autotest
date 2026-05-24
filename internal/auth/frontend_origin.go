package auth

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"autotest/internal/authprovider"
)

var ErrInvalidFrontendURL = errors.New("invalid or untrusted frontend URL")

var devTrustedOrigins = []string{
	"http://localhost:5173",
	"http://127.0.0.1:5173",
	"http://localhost:8080",
	"http://127.0.0.1:8080",
}

// NormalizeOrigin validates and normalizes a frontend origin (scheme+host+port, no path).
func NormalizeOrigin(raw string) (string, error) {
	return authprovider.NormalizeFrontendOrigin(raw)
}

func (s *Service) validateFrontendURLForProvider(raw string, trustedOrigins []string) (string, error) {
	origin, err := authprovider.NormalizeFrontendOrigin(raw)
	if err != nil {
		return "", ErrInvalidFrontendURL
	}
	if s.isDevelopment {
		for _, dev := range devTrustedOrigins {
			if origin == dev {
				return origin, nil
			}
		}
	}
	for _, allowed := range trustedOrigins {
		if origin == allowed {
			return origin, nil
		}
	}
	return "", ErrInvalidFrontendURL
}

func frontendLoginURL(origin, query string) string {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if query == "" {
		return origin + "/login"
	}
	return origin + "/login?" + query
}

func oauthSuccessRedirect(origin, jwtToken string) string {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	return fmt.Sprintf("%s/login/oauth/callback#token=%s", origin, url.QueryEscape(jwtToken))
}
