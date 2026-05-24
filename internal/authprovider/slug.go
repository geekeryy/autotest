package authprovider

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrSlugInvalid   = errors.New("slug is invalid")
	ErrSlugDuplicate = errors.New("slug already exists")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func normalizeSlug(slug string) string {
	return strings.TrimSpace(strings.ToLower(slug))
}

func validateSlug(slug string) error {
	slug = normalizeSlug(slug)
	if slug == "" {
		return fmt.Errorf("slug is required")
	}
	if !slugPattern.MatchString(slug) {
		return ErrSlugInvalid
	}
	return nil
}
