package authprovider

import "testing"

func TestValidateCallbackURL(t *testing.T) {
	t.Parallel()

	valid := "https://api.example.com/api/v1/auth/oauth/github/callback"
	if err := validateCallbackURL(valid, "github"); err != nil {
		t.Fatalf("valid url: %v", err)
	}
	if err := validateCallbackURL("", "github"); err == nil {
		t.Fatal("expected required error")
	}
	if err := validateCallbackURL("ftp://api.example.com/api/v1/auth/oauth/github/callback", "github"); err == nil {
		t.Fatal("expected scheme error")
	}
	if err := validateCallbackURL(valid, "gitlab"); err == nil {
		t.Fatal("expected slug mismatch error")
	}
	if err := validateCallbackURL("https://api.example.com/wrong/path", "github"); err == nil {
		t.Fatal("expected path error")
	}
}
