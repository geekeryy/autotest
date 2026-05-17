package aiprovider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"autotest/internal/aiprovider/client"
)

const (
	maxAssistantImagesPerMessage = 4
	maxAssistantImageBytes       = 5 * 1024 * 1024 // 5 MiB decoded
)

// MessageAttachment is a persisted user attachment (currently image only).
type MessageAttachment struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
}

// UserImageInput is an image sent with a single chat turn.
type UserImageInput struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
}

func normalizeUserImages(images []UserImageInput) ([]MessageAttachment, error) {
	if len(images) == 0 {
		return nil, nil
	}
	if len(images) > maxAssistantImagesPerMessage {
		return nil, fmt.Errorf("单条消息最多上传 %d 张图片", maxAssistantImagesPerMessage)
	}
	out := make([]MessageAttachment, 0, len(images))
	for i, img := range images {
		url := strings.TrimSpace(img.URL)
		if url == "" {
			return nil, fmt.Errorf("第 %d 张图片地址不能为空", i+1)
		}
		if err := validateAssistantImageURL(url); err != nil {
			return nil, fmt.Errorf("第 %d 张图片无效: %w", i+1, err)
		}
		out = append(out, MessageAttachment{
			Type: "image",
			URL:  url,
			Name: strings.TrimSpace(img.Name),
		})
	}
	return out, nil
}

func validateAssistantImageURL(raw string) error {
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return nil
	}
	if !strings.HasPrefix(lower, "data:") {
		return fmt.Errorf("仅支持 https 链接或 data URL")
	}
	comma := strings.Index(raw, ",")
	if comma < 0 {
		return fmt.Errorf("data URL 格式不正确")
	}
	meta := strings.ToLower(raw[:comma])
	switch {
	case strings.Contains(meta, "image/jpeg"):
	case strings.Contains(meta, "image/png"):
	case strings.Contains(meta, "image/gif"):
	case strings.Contains(meta, "image/webp"):
	default:
		return fmt.Errorf("仅支持 JPEG、PNG、GIF、WebP")
	}
	payload := raw[comma+1:]
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("base64 解码失败")
	}
	if len(decoded) > maxAssistantImageBytes {
		return fmt.Errorf("单张图片不能超过 %d MB", maxAssistantImageBytes/(1024*1024))
	}
	return nil
}

func attachmentsToRaw(atts []MessageAttachment) (json.RawMessage, error) {
	if len(atts) == 0 {
		return nil, nil
	}
	return json.Marshal(atts)
}

func attachmentsFromRaw(raw json.RawMessage) []MessageAttachment {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var atts []MessageAttachment
	if err := json.Unmarshal(raw, &atts); err != nil {
		return nil
	}
	return atts
}

func clientImagesFromAttachments(atts []MessageAttachment) []client.ImageAttachment {
	if len(atts) == 0 {
		return nil
	}
	out := make([]client.ImageAttachment, 0, len(atts))
	for _, a := range atts {
		if a.Type != "image" || strings.TrimSpace(a.URL) == "" {
			continue
		}
		out = append(out, client.ImageAttachment{URL: a.URL, Name: a.Name})
	}
	return out
}

// resolveAssistantModel picks the model id for this assistant turn.
func resolveAssistantModel(provider *providerRow, requested string) string {
	if m := strings.TrimSpace(requested); m != "" {
		return m
	}
	if provider != nil {
		if m := strings.TrimSpace(provider.DefaultModel); m != "" {
			return m
		}
	}
	return ""
}

func historyHasImageAttachments(history []StoredMessage) bool {
	for _, m := range history {
		if m.Role == "user" && len(attachmentsFromRaw(m.Attachments)) > 0 {
			return true
		}
	}
	return false
}
