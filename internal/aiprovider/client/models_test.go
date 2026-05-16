package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListModelsOpenAICompatible(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("missing auth header")
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini","owned_by":"openai"},{"id":"text-embedding-3-small","owned_by":"openai"}]}`))
	}))
	defer srv.Close()

	models, err := ListModels(context.Background(), Spec{
		Type:    "openai",
		BaseURL: srv.URL + "/v1",
		APIKey:  "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "gpt-4o-mini" {
		t.Fatalf("unexpected models: %+v", models)
	}
}

func TestParseOpenAIModelsPayloadModelsKey(t *testing.T) {
	models, err := parseOpenAIModelsPayload([]byte(`{"models":[{"name":"llama3"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "llama3" {
		t.Fatalf("unexpected: %+v", models)
	}
}

func TestListModelsAnthropic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "ak" {
			t.Fatalf("missing api key")
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-3-5-sonnet-latest","display_name":"Claude 3.5 Sonnet"}]}`))
	}))
	defer srv.Close()

	models, err := ListModels(context.Background(), Spec{
		Type:    "anthropic",
		BaseURL: srv.URL + "/v1",
		APIKey:  "ak",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "claude-3-5-sonnet-latest" {
		t.Fatalf("unexpected models: %+v", models)
	}
}

func TestListModelsOllamaNativeFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.WriteHeader(http.StatusNotFound)
		case "/api/tags":
			_, _ = w.Write([]byte(`{"models":[{"name":"llama3.1:8b"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	models, err := ListModels(context.Background(), Spec{
		Type:    "ollama",
		BaseURL: srv.URL + "/v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "llama3.1:8b" {
		t.Fatalf("unexpected models: %+v", models)
	}
}

func TestOllamaNativeBase(t *testing.T) {
	if got := ollamaNativeBase("http://127.0.0.1:11434/v1"); got != "http://127.0.0.1:11434" {
		t.Fatalf("got %q", got)
	}
}
