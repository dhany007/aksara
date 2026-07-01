package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTranslatorParsesJSONBlocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"blocks\":[{\"type\":\"paragraph\",\"text\":\"Halo.\"}]}"}}]}`))
	}))
	defer server.Close()

	translator := NewTranslator(TranslatorConfig{
		APIKey:  "sk-test",
		Model:   "deepseek-v4-pro",
		BaseURL: server.URL,
		Timeout: time.Second,
	})
	blocks, err := translator.TranslateBlocks(context.Background(), "Hello.")
	if err != nil {
		t.Fatalf("TranslateBlocks returned error: %v", err)
	}
	if len(blocks) != 1 || blocks[0].Text != "Halo." {
		t.Fatalf("blocks = %#v", blocks)
	}
}

func TestTranslatorClassifiesAuthErrorAsPermanent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad key"}}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	translator := NewTranslator(TranslatorConfig{
		APIKey:  "bad",
		Model:   "deepseek-v4-pro",
		BaseURL: server.URL,
		Timeout: time.Second,
	})
	_, err := translator.TranslateBlocks(context.Background(), "Hello.")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "permanent") {
		t.Fatalf("error = %v", err)
	}
}
