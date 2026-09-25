package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type System struct{ Folder, Cool, Fun, Games string }

var systems = []System{
	{"PSP", "psp", "playstation-portable", "playstation-portable"}, {"PS", "psx", "playstation", "playstation"},
	{"FC", "nes", "nintendo", "nintendo"}, {"SFC", "snes", "super-nintendo", "super-nintendo"},
	{"GB", "gb", "game-boy", "gameboy"}, {"GBC", "gbc", "game-boy-color", "gameboy-color"},
	{"GBA", "gba", "game-boy-advance", "gameboy-advance"}, {"N64", "n64", "nintendo-64", "nintendo-64"},
	{"NDS", "nds", "nintendo-ds", "nintendo-ds"}, {"MD", "genesis", "sega-genesis", "sega-genesis"},
	{"MS", "mastersystem", "sega-master-system", "sega-master-system"}, {"GG", "gamegear", "game-gear", "game-gear"},
	{"SS", "saturn", "sega-saturn", "sega-saturn"}, {"DC", "dc", "dreamcast", "dreamcast"},
	{"MAME", "mame", "mame", "mame-037b11"}, {"NEOGEO", "neogeo", "neo-geo", "snk-neo-geo"},
	{"CPS1", "cps1", "cps1", "capcom-play-system"}, {"CPS2", "cps2", "cps2", "capcom-play-system-2"},
	{"ATARI2600", "atari2600", "atari-2600", "atari-2600"},
	{"ATARI5200", "atari5200", "atari-5200", "atari-5200-supersystem"},
	{"ATARI7800", "atari7800", "atari-7800", "atari-7800-prosystem"},
	{"LYNX", "atarilynx", "atari-lynx", "atari-lynx"}, {"NGP", "neogeopocket", "neo-geo-pocket", "neo-geo-pocket-color"},
	{"C64", "c64", "commodore-64", "commodore-64"}, {"SEGACD", "segacd", "sega-cd", "sega-cd"},
}

type Game struct{ Source, System, Title, URL string }

var sources = []string{"CoolROM", "RomsFun", "RomsGames"}
var client = &http.Client{Transport: &http.Transport{MaxIdleConns: 8, MaxIdleConnsPerHost: 4, IdleConnTimeout: 30 * time.Second, ResponseHeaderTimeout: 25 * time.Second}}
var gameCool = regexp.MustCompile(`^/roms/([a-z0-9]+)/[0-9]+/[^/]+\.php$`)
var gameFun = regexp.MustCompile(`^/roms/([a-z0-9-]+)/[^/]+\.html$`)
var gameGames = regexp.MustCompile(`^/([a-z0-9-]+)-rom-[a-z0-9-]+/$`)

func configuredSystems(romRoot string) []System {
	var result []System
	for _, s := range systems {
		if st, e := os.Stat(filepath.Join(romRoot, s.Folder)); e != nil || !st.IsDir() {
			continue
		}
		found := false
		configs, _ := filepath.Glob(filepath.Join(filepath.Dir(romRoot), "Emus", "*", "config.json"))
		if len(configs) == 0 {
			found = true
		}
		for _, c := range configs {
			data, e := os.ReadFile(c)
			if e != nil {
				continue
			}
			var v struct {
				Rompath string `json:"rompath"`
			}
			if json.Unmarshal(data, &v) != nil || v.Rompath == "" {
				continue
			}
			p, _ := filepath.Abs(filepath.Join(filepath.Dir(c), v.Rompath))
			expected, _ := filepath.Abs(filepath.Join(romRoot, s.Folder))
			if p == expected {
				found = true
				break
			}
		}
		if found {
			result = append(result, s)
		}
	}
	return result
}
func slug(s System, source string) string {
	switch source {
	case "CoolROM":
		return s.Cool
	case "RomsFun":
		return s.Fun
	default:
		return s.Games
	}
}
func pageURL(source string, s System, page int, query string) string {
	if query != "" {
		switch source {
		case "CoolROM":
			return "https://coolrom.com/search?q=" + url.QueryEscape(query) + "&system=" + url.QueryEscape(s.Cool)
		case "RomsFun":
			return "https://romsfun.com/?s=" + url.QueryEscape(query)
		default:
			return "https://www.romsgames.net/search/?q=" + url.QueryEscape(query)
		}
	}
	switch source {
	case "CoolROM":
		if page == 1 {
			return "https://coolrom.com/roms/" + s.Cool + "/"
		}
		return fmt.Sprintf("https://coolrom.com/roms/%s/?page=%d", s.Cool, page)
	case "RomsFun":
		if page == 1 {
			return "https://romsfun.com/roms/" + s.Fun + "/"
		}
		return fmt.Sprintf("https://romsfun.com/roms/%s/page/%d/", s.Fun, page)
	default:
		return fmt.Sprintf("https://www.romsgames.net/roms/%s/?page=%d&sort=popularity", s.Games, page)
	}
}
func newRequest(method, raw, referer string, body io.Reader) (*http.Request, error) {
	req, e := http.NewRequest(method, raw, body)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux aarch64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/json,application/octet-stream;q=0.9,*/*;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}
func readPage(raw string) (string, error) {
	req, e := newRequest("GET", raw, "", nil)
	if e != nil {
		return "", e
	}
	ctx, cancel := context.WithTimeout(req.Context(), 25*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	resp, e := client.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("%s returned HTTP %d", resp.Request.URL.Hostname(), resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 2_000_001))
	if e != nil {
		return "", e
	}
	if len(b) > 2_000_000 {
		return "", fmt.Errorf("page too large")
	}
	page := string(b)
	if strings.Contains(page, "cf-chl") || strings.Contains(page, "Just a moment...") {
		return "", fmt.Errorf("site requires a browser challenge")
	}
	return page, nil
}
func links(page string) [][2]string {
	var result [][2]string
	doc, e := html.Parse(strings.NewReader(page))
	if e != nil {
		return result
	}
	var textOf func(*html.Node) string
	textOf = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}
		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.WriteString(textOf(c))
		}
		return b.String()
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					result = append(result, [2]string{a.Val, strings.TrimSpace(textOf(n))})
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return result
}
func catalog(source string, s System, page int, query string) ([]Game, bool, error) {
	raw := pageURL(source, s, page, query)
	content, e := readPage(raw)
	if e != nil {
		return nil, false, e
	}
	if source == "CoolROM" {
		content = strings.Split(content, "Top 25 Downloaded ROMs")[0]
	}
	base, _ := url.Parse(raw)
	var games []Game
	seen := map[string]bool{}
	hasNext := false
	for _, link := range links(content) {
		u, e := base.Parse(link[0])
		if e != nil {
			continue
		}
		if query == "" && u.Query().Get("page") == strconv.Itoa(page+1) && strings.Contains(u.Path, "/roms/") {
			hasNext = true
		}
		var match []string
		switch source {
		case "CoolROM":
			if u.Hostname() != "coolrom.com" {
				continue
			}
			match = gameCool.FindStringSubmatch(u.Path)
		case "RomsFun":
			if u.Hostname() != "romsfun.com" {
				continue
			}
			match = gameFun.FindStringSubmatch(u.Path)
		default:
			if u.Hostname() != "www.romsgames.net" {
				continue
			}
			match = gameGames.FindStringSubmatch(u.Path)
		}
		if len(match) < 2 || match[1] != slug(s, source) || seen[u.String()] || link[1] == "" {
			continue
		}
		seen[u.String()] = true
		games = append(games, Game{source, s.Folder, link[1], u.String()})
	}
	if len(games) == 0 {
		return nil, false, fmt.Errorf("no %s games listed for %s", source, s.Folder)
	}
	if source == "RomsFun" && query == "" {
		hasNext = strings.Contains(content, fmt.Sprintf("/page/%d/", page+1))
	}
	return games, hasNext, nil
}
