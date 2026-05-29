package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompatibleClient_Embed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var req oaEmbeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "text-embedding-3-small" {
			t.Fatalf("unexpected model %q", req.Model)
		}
		if len(req.Input) != 2 {
			t.Fatalf("expected 2 inputs, got %d", len(req.Input))
		}
		_, _ = w.Write([]byte(`{"data":[
			{"index":0,"embedding":[1,0,0]},
			{"index":1,"embedding":[0,1,0]}
		]}`))
	}))
	defer srv.Close()

	c := &OpenAICompatibleClient{BaseURL: srv.URL + "/v1", APIKey: "k"}
	vecs, err := c.Embed(context.Background(), "text-embedding-3-small", []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 || len(vecs[0]) != 3 {
		t.Fatalf("unexpected vectors: %+v", vecs)
	}
}
