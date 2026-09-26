package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
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

func TestRangeDownloadResumesInterruptedPart(t *testing.T) {
	data := make([]byte, 5<<20)
	for i := range data {
		data[i] = byte(i)
	}
	var interrupted, resumed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := strings.TrimPrefix(r.Header.Get("Range"), "bytes=")
		bounds := strings.Split(value, "-")
		if len(bounds) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		start, _ := strconv.Atoi(bounds[0])
		end := len(data) - 1
		if bounds[1] != "" {
			end, _ = strconv.Atoi(bounds[1])
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		if end > 0 && start == 0 && interrupted.CompareAndSwap(false, true) {
			_, _ = w.Write(data[:(end+1)/2])
			return
		}
		if start > 0 && start < len(data)/4 {
			resumed.Store(true)
		}
		_, _ = w.Write(data[start : end+1])
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "game.rom")
	if err := fetchFile(server.URL, server.URL, path, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !resumed.Load() || !interrupted.Load() || string(got) != string(data) {
		t.Fatalf("resume failed: interrupted=%v resumed=%v read error=%v", interrupted.Load(), resumed.Load(), err)
	}
}

func TestRomsFunFileLink(t *testing.T) {
	for _, host := range []string{"sto.romsfast.com", "statics.romsfun.com"} {
		page := `<a href="https://` + host + `/GBA/Example%20Game.zip?e=123&#038;s=abc" id="download-link">Download</a>`
		got, name, err := romsFunFileLink(page)
		if err != nil || got != "https://"+host+"/GBA/Example%20Game.zip?e=123&s=abc" || name != "Example Game.zip" {
			t.Fatalf("URL %q, name %q, error %v", got, name, err)
		}
	}
}
