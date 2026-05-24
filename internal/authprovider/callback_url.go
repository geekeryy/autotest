package authprovider

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var ErrCallbackURLInvalid = errors.New("callback url is invalid")

func validateCallbackURL(callbackURL, slug string) error {
	callbackURL = strings.TrimSpace(callbackURL)
	if callbackURL == "" {
		return fmt.Errorf("callback url is required")
	}
	u, err := url.Parse(callbackURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ErrCallbackURLInvalid
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrCallbackURLInvalid
	}
	slug = normalizeSlug(slug)
	expectedPath := "/api/v1/auth/oauth/" + slug + "/callback"
	path := strings.TrimRight(u.Path, "/")
	if path != expectedPath {
		return ErrCallbackURLInvalid
	}
	return nil
}
