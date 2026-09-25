package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSmallRangeDownloadRequestsWholeFile(t *testing.T) {
	data := []byte("small playable ROM payload")
	for _, fullStatus := range []int{http.StatusOK, http.StatusPartialContent} {
		t.Run(fmt.Sprint(fullStatus), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Range") == "bytes=0-0" {
					w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", len(data)))
					w.WriteHeader(http.StatusPartialContent)
					_, _ = w.Write(data[:1])
					return
				}
				if fullStatus == http.StatusPartialContent {
					w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(data)-1, len(data)))
				}
				w.WriteHeader(fullStatus)
				_, _ = w.Write(data)
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "game.rom")
			if err := fetchFile(server.URL, server.URL, path, nil); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != string(data) {
				t.Fatalf("downloaded %q, error %v", got, err)
			}
		})
	}
}
