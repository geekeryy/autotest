package authprovider

import "testing"

func TestNormalizeFrontendOrigin(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"https://app.example.com/", "https://app.example.com", true},
		{"http://localhost:8080", "http://localhost:8080", true},
		{"https://app.example.com/path", "", false},
		{"ftp://app.example.com", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, err := NormalizeFrontendOrigin(tc.in)
		if tc.ok && err != nil {
			t.Fatalf("NormalizeFrontendOrigin(%q): %v", tc.in, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("NormalizeFrontendOrigin(%q) expected error", tc.in)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("NormalizeFrontendOrigin(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeTrustedFrontendOriginsDedupes(t *testing.T) {
	t.Parallel()

	got, err := normalizeTrustedFrontendOrigins([]string{
		"https://app.example.com/",
		"https://app.example.com",
		"http://localhost:8080",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}
