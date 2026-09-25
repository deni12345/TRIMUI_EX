package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrowserChallenge403(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<title>Just a moment...</title>"))
	}))
	defer server.Close()
	_, err := readPageContext(context.Background(), server.URL)
	if !errors.Is(err, errBrowserChallenge) {
		t.Fatalf("expected browser challenge, got %v", err)
	}
}

func TestNormalPageWithCloudflareScript(t *testing.T) {
	page := `<html><script src="/cdn-cgi/challenge-platform/scripts/jsd/main.js"></script><a href="/roms/">Games</a></html>`
	got, err := checkedPage("romsfun.com", http.StatusOK, []byte(page))
	if err != nil || got != page {
		t.Fatalf("normal page rejected: %v", err)
	}
}
