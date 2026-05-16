package auth

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// 1×1 PNG（透明像素）
const tinyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func TestProcessAvatarBytes(t *testing.T) {
	raw, err := base64.StdEncoding.DecodeString(tinyPNG)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ProcessAvatarBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 100 || len(out) > 50_000 {
		t.Fatalf("unexpected jpeg size: %d", len(out))
	}
	if !bytes.HasPrefix(out, []byte{0xff, 0xd8, 0xff}) {
		t.Fatal("output is not JPEG")
	}
}

func TestProcessAvatarBase64_dataURL(t *testing.T) {
	b64 := "data:image/png;base64," + tinyPNG
	out, err := ProcessAvatarBase64(b64)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 50 {
		t.Fatal("expected non-empty jpeg")
	}
}
