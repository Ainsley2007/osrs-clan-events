package proofstorage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPersistFromURL_SetsContentLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(minPNG)
	}))
	t.Cleanup(server.Close)

	store := &R2Store{
		httpClient: server.Client(),
	}

	data, contentType, err := store.download(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if len(data) != len(minPNG) {
		t.Fatalf("download len = %d, want %d", len(data), len(minPNG))
	}
	if contentType != "image/png" {
		t.Fatalf("content type = %q, want image/png", contentType)
	}
}
