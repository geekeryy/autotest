package client

import "encoding/json"

// ImageAttachment is a user-provided image for multimodal chat (OpenAI
// image_url content parts).
type ImageAttachment struct {
	URL  string
	Name string
}

type oaContentPart struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	ImageURL *oaImageURLRef `json:"image_url,omitempty"`
}

type oaImageURLRef struct {
	URL string `json:"url"`
}

// encodeOAContent serialises message content for OpenAI-compatible APIs.
// Plain text becomes a JSON string; text + images become a content-parts array.
func encodeOAContent(text string, images []ImageAttachment) (json.RawMessage, error) {
	if len(images) == 0 {
		return json.Marshal(text)
	}
	parts := make([]oaContentPart, 0, 1+len(images))
	if text != "" {
		parts = append(parts, oaContentPart{Type: "text", Text: text})
	}
	for _, img := range images {
		parts = append(parts, oaContentPart{
			Type:     "image_url",
			ImageURL: &oaImageURLRef{URL: img.URL},
		})
	}
	if len(parts) == 0 {
		return json.Marshal("")
	}
	return json.Marshal(parts)
}
