package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "golang.org/x/image/webp"
)

const coverWidth, coverHeight = 94, 116

var coverDirOnce sync.Once
var cachedCoverDir string

func coverCacheDir() string {
	coverDirOnce.Do(func() {
		if override := os.Getenv("ROM_SEARCH_COVER_CACHE"); override != "" {
			cachedCoverDir = override
		} else if st, e := os.Stat("/mnt/UDISK"); e == nil && st.IsDir() && freeBytes("/mnt/UDISK") > 20<<20 {
			cachedCoverDir = "/mnt/UDISK/romsearch-covers"
		} else {
			cachedCoverDir = filepath.Join(appDir(), "cache")
		}
	})
	return cachedCoverDir
}

func coverPath(game Game) string {
	if game.ImageURL == "" {
		return ""
	}
	digest := sha1.Sum([]byte(game.ImageURL))
	return filepath.Join(coverCacheDir(), hex.EncodeToString(digest[:])+".png")
}

func permittedCoverHost(host string) bool {
	switch host {
	case "cache.downloadroms.io", "static.downloadroms.io", "cache.lategames.net", "coolrom.com", "romsfun.com":
		return true
	}
	return false
}

func fetchCover(game Game) (string, error) {
	target := coverPath(game)
	if target == "" {
		return "", fmt.Errorf("game has no cover URL")
	}
	if info, e := os.Stat(target); e == nil && info.Size() > 0 {
		return target, nil
	}
	u, e := url.Parse(game.ImageURL)
	if e != nil || u.Scheme != "https" || !permittedCoverHost(u.Hostname()) {
		return "", fmt.Errorf("unsupported cover host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
	defer cancel()
	req, e := newRequest("GET", game.ImageURL, game.URL, nil)
	if e != nil {
		return "", e
	}
	resp, e := client.Do(req.WithContext(ctx))
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Request.URL.Scheme != "https" || !permittedCoverHost(resp.Request.URL.Hostname()) {
		return "", fmt.Errorf("cover HTTP %d", resp.StatusCode)
	}
	payload, e := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if e != nil {
		return "", e
	}
	if len(payload) > 1<<20 {
		return "", fmt.Errorf("cover exceeds size limit")
	}
	source, _, e := image.Decode(bytes.NewReader(payload))
	if e != nil {
		return "", e
	}
	bounds := source.Bounds()
	if bounds.Dx() < 1 || bounds.Dy() < 1 || bounds.Dx() > 4096 || bounds.Dy() > 4096 {
		return "", fmt.Errorf("invalid cover dimensions")
	}
	scale := min(float64(coverWidth)/float64(bounds.Dx()), float64(coverHeight)/float64(bounds.Dy()))
	width := max(1, int(float64(bounds.Dx())*scale))
	height := max(1, int(float64(bounds.Dy())*scale))
	out := image.NewRGBA(image.Rect(0, 0, coverWidth, coverHeight))
	offsetX, offsetY := (coverWidth-width)/2, (coverHeight-height)/2
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sx := bounds.Min.X + x*bounds.Dx()/width
			sy := bounds.Min.Y + y*bounds.Dy()/height
			out.Set(offsetX+x, offsetY+y, source.At(sx, sy))
		}
	}
	if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
		return "", e
	}
	temporary, e := os.CreateTemp(filepath.Dir(target), ".cover-")
	if e != nil {
		return "", e
	}
	defer os.Remove(temporary.Name())
	if e = png.Encode(temporary, out); e != nil {
		temporary.Close()
		return "", e
	}
	if e = temporary.Close(); e != nil {
		return "", e
	}
	if e = os.Rename(temporary.Name(), target); e != nil {
		return "", e
	}
	return target, nil
}
