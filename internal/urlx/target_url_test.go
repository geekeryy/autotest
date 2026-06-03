package urlx

import "testing"

func TestValidateHTTPServiceURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "empty", url: "", wantErr: true},
		{name: "file scheme", url: "file:///etc/passwd", wantErr: true},
		{name: "localhost", url: "http://localhost:8080", wantErr: true},
		{name: "private ip", url: "http://192.168.1.10/api", wantErr: true},
		{name: "loopback ip", url: "http://127.0.0.1/api", wantErr: true},
		{name: "public https", url: "https://1.1.1.1/health", wantErr: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateHTTPServiceURL(tc.url)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
