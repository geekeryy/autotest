package auth

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
	"strings"

	"golang.org/x/image/draw"
)

const (
	avatarMaxEdge    = 256
	avatarJPEGQ      = 82
	maxRawImageBytes = 8 << 20 // 上传解码前上限
)

// ProcessAvatarBase64 解码 base64（可为标准 base64 或 data URL），缩放并编码为 JPEG。
func ProcessAvatarBase64(b64 string) ([]byte, error) {
	b64 = strings.TrimSpace(b64)
	if b64 == "" {
		return nil, errors.New("头像数据为空")
	}
	if i := strings.Index(b64, "base64,"); i >= 0 {
		b64 = strings.TrimSpace(b64[i+len("base64,"):])
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("头像 base64 无效: %w", err)
	}
	return ProcessAvatarBytes(raw)
}

// ProcessAvatarBytes 将任意支持的栅格图缩放为不超过 avatarMaxEdge 的 JPEG。
func ProcessAvatarBytes(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("头像数据为空")
	}
	if len(raw) > maxRawImageBytes {
		return nil, errors.New("图片过大，请选择较小的文件")
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("无法识别图片格式（支持 JPEG/PNG/GIF）: %w", err)
	}
	out, err := encodeAvatarJPEG(img)
	if err != nil {
		return nil, err
	}
	if len(out) > 512<<10 {
		return nil, errors.New("压缩后头像仍过大，请换一张图片")
	}
	return out, nil
}

func encodeAvatarJPEG(src image.Image) ([]byte, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, errors.New("图片尺寸无效")
	}
	scale := 1.0
	if fw := float64(avatarMaxEdge) / float64(w); fw < scale {
		scale = fw
	}
	if fh := float64(avatarMaxEdge) / float64(h); fh < scale {
		scale = fh
	}
	nw := int(float64(w)*scale + 0.5)
	nh := int(float64(h)*scale + 0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: avatarJPEGQ}); err != nil {
		return nil, fmt.Errorf("编码头像: %w", err)
	}
	return buf.Bytes(), nil
}
