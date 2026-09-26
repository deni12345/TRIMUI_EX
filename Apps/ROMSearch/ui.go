package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ebitengine/purego"
)

var sdl struct {
	Init                     func(uint32) int32
	Quit                     func()
	CreateWindow             func(string, int32, int32, int32, int32, uint32) uintptr
	DestroyWindow            func(uintptr)
	CreateRenderer           func(uintptr, int32, uint32) uintptr
	DestroyRenderer          func(uintptr)
	RenderSetLogicalSize     func(uintptr, int32, int32) int32
	SetRenderDrawColor       func(uintptr, uint8, uint8, uint8, uint8) int32
	RenderClear              func(uintptr) int32
	RenderPresent            func(uintptr)
	RenderCopy               func(uintptr, uintptr, *sdlRect, *sdlRect) int32
	RenderReadPixels         func(uintptr, *sdlRect, uint32, *byte, int32) int32
	DestroyTexture           func(uintptr)
	SetHint                  func(string, string) int32
	PollEvent                func(*byte) int32
	GetError                 func() string
	NumJoysticks             func() int32
	GameControllerOpen       func(int32) uintptr
	GameControllerClose      func(uintptr)
	GameControllerEventState func(int32) int32
	JoystickEventState       func(int32) int32
	Box                      func(uintptr, int32, int32, int32, int32, uint8, uint8, uint8, uint8) int32
	String                   func(uintptr, int32, int32, string, uint8, uint8, uint8, uint8) int32
	LoadTexture              func(uintptr, string) uintptr
}

type sdlRect struct{ X, Y, W, H int32 }

func bindSDL() error {
	lib, e := purego.Dlopen("libSDL2-2.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if e != nil {
		return e
	}
	gfx, e := purego.Dlopen("libSDL2_gfx-1.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if e != nil {
		return e
	}
	purego.RegisterLibFunc(&sdl.Init, lib, "SDL_Init")
	purego.RegisterLibFunc(&sdl.Quit, lib, "SDL_Quit")
	purego.RegisterLibFunc(&sdl.CreateWindow, lib, "SDL_CreateWindow")
	purego.RegisterLibFunc(&sdl.DestroyWindow, lib, "SDL_DestroyWindow")
	purego.RegisterLibFunc(&sdl.CreateRenderer, lib, "SDL_CreateRenderer")
	purego.RegisterLibFunc(&sdl.DestroyRenderer, lib, "SDL_DestroyRenderer")
	purego.RegisterLibFunc(&sdl.RenderSetLogicalSize, lib, "SDL_RenderSetLogicalSize")
	purego.RegisterLibFunc(&sdl.SetRenderDrawColor, lib, "SDL_SetRenderDrawColor")
	purego.RegisterLibFunc(&sdl.RenderClear, lib, "SDL_RenderClear")
	purego.RegisterLibFunc(&sdl.RenderPresent, lib, "SDL_RenderPresent")
	purego.RegisterLibFunc(&sdl.RenderCopy, lib, "SDL_RenderCopy")
	purego.RegisterLibFunc(&sdl.RenderReadPixels, lib, "SDL_RenderReadPixels")
	purego.RegisterLibFunc(&sdl.DestroyTexture, lib, "SDL_DestroyTexture")
	purego.RegisterLibFunc(&sdl.SetHint, lib, "SDL_SetHint")
	purego.RegisterLibFunc(&sdl.PollEvent, lib, "SDL_PollEvent")
	purego.RegisterLibFunc(&sdl.GetError, lib, "SDL_GetError")
	purego.RegisterLibFunc(&sdl.NumJoysticks, lib, "SDL_NumJoysticks")
	purego.RegisterLibFunc(&sdl.GameControllerOpen, lib, "SDL_GameControllerOpen")
	purego.RegisterLibFunc(&sdl.GameControllerClose, lib, "SDL_GameControllerClose")
	purego.RegisterLibFunc(&sdl.GameControllerEventState, lib, "SDL_GameControllerEventState")
	purego.RegisterLibFunc(&sdl.JoystickEventState, lib, "SDL_JoystickEventState")
	purego.RegisterLibFunc(&sdl.Box, gfx, "boxRGBA")
	purego.RegisterLibFunc(&sdl.String, gfx, "stringRGBA")
	imageLib, e := purego.Dlopen("libSDL2_image-2.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if e != nil {
		return e
	}
	purego.RegisterLibFunc(&sdl.LoadTexture, imageLib, "IMG_LoadTexture")
	return nil
}

type keyboardKey struct {
	label, value string
	width        int32
}

func letterKeys(row string) []keyboardKey {
	keys := make([]keyboardKey, 0, len(row))
	for _, letter := range row {
		keys = append(keys, keyboardKey{string(letter), strings.ToLower(string(letter)), 45})
	}
	return keys
}

var letterRows = [][]keyboardKey{
	letterKeys("QWERTYUIOP"),
	letterKeys("ASDFGHJKL"),
	append(append([]keyboardKey{{"SHIFT", "shift", 55}}, letterKeys("ZXCVBNM")...), keyboardKey{"DEL", "delete", 55}),
	{{"123", "symbols", 55}, {",", ",", 40}, {"SPACE", " ", 250}, {".", ".", 40}, {"GO", "submit", 55}},
}
var symbolRows = [][]keyboardKey{
	letterKeys("1234567890"),
	letterKeys("@#$%&*()-"),
	{{"ABC", "symbols", 55}, {"/", "/", 45}, {":", ":", 45}, {";", ";", 45}, {"'", "'", 45}, {"\"", "\"", 45}, {"?", "?", 45}, {"!", "!", 45}, {"DEL", "delete", 55}},
	{{"ABC", "symbols", 55}, {",", ",", 40}, {"SPACE", " ", 250}, {".", ".", 40}, {"GO", "submit", 55}},
}

type result struct {
	kind        string
	id          int
	key         string
	games       []Game
	systemIndex int
	pageNumber  int
	hasNext     bool
	err         error
}
type catalogPage struct {
	games      []Game
	pages      []int
	nextSystem int
	hasMore    bool
}
type App struct {
	source           int
	displaySource    int
	systems          []System
	query            string
	games            []Game
	selected         int
	mode             string
	keyX, keyY       int
	symbols, shift   bool
	busy             bool
	loading          bool
	browseID         int
	browseCancel     context.CancelFunc
	moreCancel       context.CancelFunc
	moreBusy         bool
	pendingAdvance   bool
	pendingIndex     int
	pages            []int
	nextSystem       int
	hasMore          bool
	status           string
	showStatus       bool
	catalogError     bool
	messages         chan result
	romRoot, imgRoot string
	running          bool
	cache            map[string]catalogPage
	coverPending     map[string]bool
	coverFailed      map[string]bool
	coverSlots       chan struct{}
	textures         map[string]uintptr
	captured         bool
	preloadPage      int
	preloadIndex     int
	job              *installJob
	jobPollAt        time.Time
}

func newApp() *App {
	started := time.Now()
	root := os.Getenv("ROM_SEARCH_ROOT")
	if root == "" {
		root = "/mnt/SDCARD/Roms"
	}
	img := os.Getenv("ROM_SEARCH_IMG_ROOT")
	if img == "" {
		img = "/mnt/SDCARD/Imgs"
	}
	a := &App{source: 2, displaySource: 2, systems: configuredSystems(root), romRoot: root, imgRoot: img, mode: "results", shift: true, messages: make(chan result, 100), cache: make(map[string]catalogPage), coverPending: make(map[string]bool), coverFailed: make(map[string]bool), coverSlots: make(chan struct{}, 2), textures: make(map[string]uintptr), preloadPage: -1, running: true}
	if job, err := readInstallJob(); err == nil {
		a.job = job
		if job.State == "done" {
			a.status, a.showStatus = "Installed "+filepath.Base(job.Path), true
		} else if job.State == "error" {
			a.status, a.showStatus = job.Error, true
		}
	}
	log.Printf("startup: emulator scan %s, %d systems", time.Since(started), len(a.systems))
	if len(a.systems) == 0 {
		a.status = "No configured emulator ROM folders"
		return a
	}
	a.games = featuredGames(a.systems)
	a.pages = make([]int, len(a.systems))
	a.hasMore = true
	a.cache["RomsGames|"] = catalogPage{games: a.games, pages: a.pages, hasMore: true}
	if !a.showStatus {
		a.status = fmt.Sprintf("%d featured games  |  SELECT type", len(a.games))
	}
	return a
}
func (a *App) browse() {
	if a.busy || len(a.systems) == 0 {
		return
	}
	if a.moreCancel != nil {
		a.moreCancel()
		a.moreCancel = nil
	}
	a.moreBusy = false
	a.pendingAdvance = false
	a.pendingIndex = 0
	a.browseID++
	source, query := sources[a.source], a.query
	key := source + "|" + query
	if cached, found := a.cache[key]; found {
		a.games = cached.games
		a.pages = cached.pages
		a.nextSystem = cached.nextSystem
		a.hasMore = cached.hasMore
		a.displaySource = a.source
		a.showStatus = false
		a.catalogError = false
		a.preloadPage = -1
		a.selected = 0
		a.mode = "results"
		a.status = fmt.Sprintf("%d games  |  SELECT type", len(cached.games))
		return
	}
	a.busy = true
	a.loading = true
	a.games = nil
	a.pages = nil
	a.hasMore = false
	a.selected = 0
	a.preloadPage = -1
	a.status = "Loading games... X switches, B cancels"
	a.showStatus = false
	a.catalogError = false
	ctx, cancel := context.WithCancel(context.Background())
	a.browseCancel = cancel
	id := a.browseID
	go func() {
		started := time.Now()
		games, e := catalogMixed(ctx, source, query, a.systems)
		log.Printf("catalog: %s query %q in %s, %d games, error=%v", source, query, time.Since(started), len(games), e)
		a.messages <- result{kind: "browse", id: id, key: key, games: games, err: e}
		log.Printf("catalog queued: id=%d source=%s", id, source)
	}()
}
func (a *App) loadMore() {
	if a.busy || a.moreBusy || !a.hasMore || a.query != "" || len(a.games) == 0 {
		return
	}
	for tried := 0; tried < len(a.systems); tried++ {
		index := (a.nextSystem + tried) % len(a.systems)
		if a.pages[index] < 0 {
			continue
		}
		a.nextSystem = (index + 1) % len(a.systems)
		a.moreBusy = true
		ctx, cancel := context.WithCancel(context.Background())
		a.moreCancel = cancel
		id, number, system, source := a.browseID, a.pages[index]+1, a.systems[index], sources[a.source]
		go func() {
			games, next, err := catalogSystemPage(ctx, source, system, number)
			log.Printf("catalog page: %s %s page %d, %d games, next=%v, error=%v", source, system.Folder, number, len(games), next, err)
			a.messages <- result{kind: "more", id: id, games: games, systemIndex: index, pageNumber: number, hasNext: next, err: err}
		}()
		return
	}
	a.hasMore = false
	a.pendingAdvance = false
	a.cache[sources[a.source]+"|"+a.query] = catalogPage{games: a.games, pages: a.pages, nextSystem: a.nextSystem, hasMore: false}
}
func (a *App) maybeLoadMore() {
	if a.mode == "results" && len(a.games) > 0 && a.selected >= len(a.games)-8 {
		a.loadMore()
	}
}
func (a *App) cancelLoading() {
	if a.browseCancel != nil {
		a.browseCancel()
		a.browseCancel = nil
	}
	a.browseID++
	if a.moreCancel != nil {
		a.moreCancel()
		a.moreCancel = nil
	}
	a.moreBusy = false
	a.busy = false
	a.loading = false
	a.mode = "results"
	a.status = "Loading cancelled"
	a.showStatus = true
	a.catalogError = false
}
func (a *App) install() {
	if a.loading || len(a.games) == 0 {
		return
	}
	job, err := startInstallJob(a.games[a.selected], a.romRoot, a.imgRoot)
	if err != nil {
		a.status, a.showStatus = err.Error(), true
		return
	}
	a.job = job
	a.status, a.showStatus = "Download started; browse or exit freely", true
}
func (a *App) poll() bool {
	changed := false
	for {
		select {
		case r := <-a.messages:
			changed = true
			switch r.kind {
			case "browse":
				log.Printf("catalog received: id=%d current=%d error=%v", r.id, a.browseID, r.err)
				if r.id != a.browseID {
					continue
				}
				a.busy = false
				a.loading = false
				a.browseCancel = nil
				if r.err != nil {
					a.status = sources[a.source] + ": " + r.err.Error()
					a.showStatus = true
					a.catalogError = true
				} else {
					a.pages = make([]int, len(a.systems))
					a.nextSystem = 0
					a.hasMore = a.query == ""
					a.cache[r.key] = catalogPage{games: r.games, pages: a.pages, hasMore: a.hasMore}
					a.mode = "results"
					a.games = r.games
					a.displaySource = a.source
					a.showStatus = false
					a.catalogError = false
					a.preloadPage = -1
					a.selected = 0
					a.status = fmt.Sprintf("%d games  |  SELECT type", len(r.games))
				}
			case "more":
				if r.id != a.browseID {
					continue
				}
				a.moreBusy = false
				a.moreCancel = nil
				if r.err != nil {
					log.Printf("catalog page skipped: %v", r.err)
				}
				if r.err != nil || len(r.games) == 0 || !r.hasNext {
					a.pages[r.systemIndex] = -1
				} else {
					a.pages[r.systemIndex] = r.pageNumber
				}
				seen := make(map[string]bool, len(a.games))
				for _, game := range a.games {
					seen[game.URL] = true
				}
				oldCount := len(a.games)
				for _, game := range r.games {
					if !seen[game.URL] {
						a.games = append(a.games, game)
						seen[game.URL] = true
					}
				}
				if a.pendingAdvance && len(a.games) > oldCount {
					a.selected = min(len(a.games)-1, a.pendingIndex)
					a.pendingAdvance = a.selected < a.pendingIndex
				}
				a.cache[sources[a.source]+"|"+a.query] = catalogPage{games: a.games, pages: a.pages, nextSystem: a.nextSystem, hasMore: a.hasMore}
				if (len(a.games) == oldCount || a.pendingAdvance) && a.hasMore {
					a.loadMore()
				}
			case "cover":
				delete(a.coverPending, r.key)
				if r.err != nil {
					a.coverFailed[r.key] = true
					log.Printf("cover: %s: %v", r.key, r.err)
				} else {
					a.preloadPage = -1
				}
			}
		default:
			if time.Since(a.jobPollAt) >= 250*time.Millisecond {
				a.jobPollAt = time.Now()
				if job, err := readInstallJob(); err == nil {
					if (job.State == "queued" || job.State == "running") && !jobActive(job) {
						job.State, job.Error, job.Stage = "error", "Download stopped unexpectedly", "Download failed"
						_ = writeInstallJob(job)
					}
					if a.job != nil && job.Updated == a.job.Updated {
						return changed
					}
					a.job = job
					changed = true
					if job.State == "done" {
						a.status, a.showStatus = "Installed "+filepath.Base(job.Path), true
					} else if job.State == "error" {
						a.status, a.showStatus = job.Error, true
					}
				}
			}
			return changed
		}
	}
}
func (a *App) ensureCovers() {
	if a.mode != "results" || len(a.games) == 0 {
		return
	}
	top := (a.selected / 8) * 8
	for i := top; i < len(a.games) && i < top+8; i++ {
		game := a.games[i]
		target := coverPath(game)
		if target == "" || a.coverPending[target] || a.coverFailed[target] {
			continue
		}
		if info, e := os.Stat(target); e == nil && info.Size() > 0 {
			continue
		}
		a.coverPending[target] = true
		go func() {
			a.coverSlots <- struct{}{}
			_, e := fetchCover(game)
			<-a.coverSlots
			a.messages <- result{kind: "cover", key: target, err: e}
		}()
	}
}
func (a *App) coverTexture(game Game) uintptr {
	target := coverPath(game)
	if target == "" {
		return 0
	}
	return a.textures[target]
}
func (a *App) preloadTexture(renderer uintptr, game Game) bool {
	target := coverPath(game)
	if target == "" {
		return false
	}
	if _, found := a.textures[target]; found {
		return false
	}
	if info, e := os.Stat(target); e != nil || info.Size() == 0 {
		return false
	}
	texture := sdl.LoadTexture(renderer, target)
	a.textures[target] = texture
	return texture != 0
}
func (a *App) closeTextures() {
	for _, texture := range a.textures {
		if texture != 0 {
			sdl.DestroyTexture(texture)
		}
	}
}
func (a *App) preloadNearby(renderer uintptr) bool {
	if a.mode != "results" || len(a.games) == 0 {
		return false
	}
	page := a.selected / 8
	if page != a.preloadPage {
		keep := make(map[string]bool)
		for i := max(0, (page-1)*8); i < len(a.games) && i < (page+2)*8; i++ {
			keep[coverPath(a.games[i])] = true
		}
		for path, texture := range a.textures {
			if !keep[path] {
				if texture != 0 {
					sdl.DestroyTexture(texture)
				}
				delete(a.textures, path)
			}
		}
		a.preloadPage = page
		a.preloadIndex = page * 8
	}
	if a.preloadIndex < len(a.games) && a.preloadIndex < (page+2)*8 {
		loaded := a.preloadTexture(renderer, a.games[a.preloadIndex])
		a.preloadIndex++
		return loaded
	}
	return false
}
func (a *App) keyRows() [][]keyboardKey {
	if a.symbols {
		return symbolRows
	}
	return letterRows
}
func (a *App) typeSelectedKey() {
	key := a.keyRows()[a.keyY][a.keyX]
	switch key.value {
	case "delete":
		if len(a.query) > 0 {
			a.query = a.query[:len(a.query)-1]
		}
	case "symbols":
		a.symbols = !a.symbols
		a.keyX = min(a.keyX, len(a.keyRows()[a.keyY])-1)
	case "shift":
		a.shift = !a.shift
	case "submit":
		a.press("search")
	default:
		if len(a.query) < 100 {
			value := key.value
			if a.shift && !a.symbols {
				value = strings.ToUpper(value)
				a.shift = false
			}
			a.query += value
		}
	}
}
func (a *App) moveKeyVertical(step int) {
	oldRows := a.keyRows()
	oldRow := oldRows[a.keyY]
	oldX := a.keyX
	a.keyY = max(0, min(len(oldRows)-1, a.keyY+step))
	newRow := a.keyRows()[a.keyY]
	if len(oldRow) > 1 {
		a.keyX = int(float64(oldX)*float64(len(newRow)-1)/float64(len(oldRow)-1) + 0.5)
	}
	a.keyX = max(0, min(len(newRow)-1, a.keyX))
}
func (a *App) press(action string) {
	if action == "quit" {
		if a.loading {
			a.cancelLoading()
		}
		a.running = false
		return
	}
	if a.loading {
		switch action {
		case "source", "back", "edit", "search":
			a.cancelLoading()
			if action == "back" {
				return
			}
		case "accept":
			return
		}
	}
	switch action {
	case "source":
		a.source = (a.source + 1) % len(sources)
		a.query = ""
		a.selected = 0
		a.mode = "results"
		a.browse()
	case "edit":
		if a.mode == "keyboard" {
			a.mode = "results"
		} else {
			a.mode = "keyboard"
			a.status = "Type a name, then choose GO"
		}
	case "search":
		if len(strings.TrimSpace(a.query)) < 3 {
			a.mode = "keyboard"
			a.status = "Enter at least 3 letters"
			return
		}
		a.mode = "results"
		a.browse()
	case "back":
		if a.mode == "keyboard" {
			if len(a.query) > 0 {
				a.query = a.query[:len(a.query)-1]
			} else {
				a.mode = "results"
			}
		} else if a.query != "" {
			a.query = ""
			a.browse()
		}
	case "accept":
		if a.mode == "keyboard" {
			a.typeSelectedKey()
		} else if len(a.games) > 0 {
			a.install()
		}
	case "up", "down":
		step := 4
		if action == "up" {
			step = -4
		}
		if a.mode == "keyboard" {
			if step < 0 {
				step = -1
			} else {
				step = 1
			}
			a.moveKeyVertical(step)
		} else if len(a.games) > 0 {
			if step > 0 && a.selected+step >= len(a.games) && a.hasMore {
				a.pendingAdvance = true
				a.pendingIndex = a.selected + step
			}
			a.selected = max(0, min(len(a.games)-1, a.selected+step))
			a.maybeLoadMore()
		}
	case "left", "right", "pagePrev", "pageNext":
		step := 1
		if action == "left" {
			step = -1
		}
		if action == "pagePrev" {
			step = -8
		}
		if action == "pageNext" {
			step = 8
		}
		if a.mode == "keyboard" {
			if step < 0 {
				step = -1
			} else {
				step = 1
			}
			a.keyX = max(0, min(len(a.keyRows()[a.keyY])-1, a.keyX+step))
		} else if len(a.games) > 0 {
			if step > 0 && a.selected+step >= len(a.games) && a.hasMore {
				a.pendingAdvance = true
				a.pendingIndex = a.selected + step
			}
			a.selected = max(0, min(len(a.games)-1, a.selected+step))
			a.maybeLoadMore()
		}
	}
}
func panel(r uintptr, x, y, w, h int32, c [3]uint8) {
	sdl.Box(r, x, y, x+w, y+h, c[0], c[1], c[2], 255)
}
func label(r uintptr, x, y int32, value string, c [3]uint8) {
	var b strings.Builder
	for _, ch := range value {
		if ch >= 32 && ch <= 126 {
			b.WriteRune(ch)
		} else {
			b.WriteByte('?')
		}
	}
	s := b.String()
	if len(s) > 61 {
		s = s[:61]
	}
	sdl.String(r, x, y, s, c[0], c[1], c[2], 255)
}
func shortTitle(title string, limit int) string {
	runes := []rune(title)
	if len(runes) <= limit {
		return title
	}
	return string(runes[:limit-1]) + "~"
}
func badgeLabel(folder string) string {
	switch folder {
	case "ATARI2600":
		return "A2600"
	case "ATARI5200":
		return "A5200"
	case "ATARI7800":
		return "A7800"
	}
	if len(folder) > 6 {
		return folder[:6]
	}
	return folder
}

func buttonAction(kind uint32, button uint8) string {
	// SDL's controller mapping can omit a device button. The matching raw
	// joystick buttons provide a fallback on the Brick Pro's virtual pad.
	switch kind {
	case 0x651: // SDL_CONTROLLERBUTTONDOWN
		switch button {
		case 0:
			return "back"
		case 1:
			return "accept"
		case 2:
			return "edit"
		case 3:
			return "source"
		case 4:
			return "edit" // SELECT
		case 6:
			return "quit" // START
		case 9:
			return "pagePrev"
		case 10:
			return "pageNext"
		case 11:
			return "up"
		case 12:
			return "down"
		case 13:
			return "left"
		case 14:
			return "right"
		}
	case 0x603: // SDL_JOYBUTTONDOWN, X360 raw button order
		switch button {
		case 6:
			return "edit" // SELECT
		case 7:
			return "quit" // START
		}
	}
	return ""
}

func physicalButton(kind uint32, button uint8) uint8 {
	if kind == 0x651 {
		switch button {
		case 4:
			return 1 // SELECT
		case 6:
			return 2 // START
		}
	}
	if kind == 0x603 {
		switch button {
		case 6:
			return 1
		case 7:
			return 2
		}
	}
	return 0
}

func (a *App) render(r uintptr) {
	sdl.SetRenderDrawColor(r, 14, 19, 31, 255)
	sdl.RenderClear(r)
	panel(r, 0, 0, 512, 43, [3]uint8{35, 52, 76})
	label(r, 10, 7, "ROM SEARCH", [3]uint8{120, 214, 255})
	label(r, 224, 7, strings.ToUpper(sources[a.source]), [3]uint8{230, 235, 245})
	totalPages := max(1, (len(a.games)+7)/8)
	currentPage := a.selected/8 + 1
	count := fmt.Sprintf("%d GAMES %d/%d", len(a.games), currentPage, totalPages)
	if a.hasMore {
		count = fmt.Sprintf("%d GAMES %d/+", len(a.games), currentPage)
	}
	if len(a.games) == 0 {
		count = "0 GAMES"
	}
	label(r, 376, 7, count, [3]uint8{210, 223, 239})
	searchBorder := [3]uint8{75, 94, 118}
	if a.mode == "keyboard" {
		searchBorder = [3]uint8{70, 180, 226}
	}
	panel(r, 8, 23, 496, 19, searchBorder)
	panel(r, 10, 25, 492, 15, [3]uint8{19, 29, 44})
	searchText := a.query
	if searchText == "" {
		searchText = "GAME TITLE"
	} else if a.mode == "keyboard" {
		searchText += "_"
	}
	if len(searchText) > 46 {
		searchText = searchText[len(searchText)-46:]
	}
	label(r, 15, 29, "SEARCH: "+searchText, [3]uint8{210, 223, 239})
	if a.mode == "keyboard" {
		label(r, 10, 55, "A TYPE   B DELETE   GO SEARCH   SELECT CLOSE", [3]uint8{210, 223, 239})
		panel(r, 0, 85, 512, 260, [3]uint8{193, 198, 207})
		label(r, 20, 95, "SEARCH GAMES", [3]uint8{53, 60, 70})
		for y, row := range a.keyRows() {
			var rowWidth int32
			for _, key := range row {
				rowWidth += key.width + 4
			}
			rowWidth -= 4
			px, py := (int32(512)-rowWidth)/2, int32(119+y*54)
			for x, key := range row {
				panel(r, px+1, py+3, key.width, 45, [3]uint8{151, 157, 166})
				color, ink := [3]uint8{250, 251, 252}, [3]uint8{35, 43, 55}
				if key.value == "shift" || key.value == "delete" || key.value == "symbols" {
					color = [3]uint8{170, 178, 190}
				}
				if x == a.keyX && y == a.keyY {
					color, ink = [3]uint8{41, 129, 222}, [3]uint8{255, 255, 255}
				}
				panel(r, px, py, key.width, 43, color)
				label(r, px+(key.width-int32(len(key.label)*8))/2, py+17, key.label, ink)
				px += key.width + 4
			}
		}
	} else {
		a.ensureCovers()
		if len(a.games) == 0 {
			message := "NO GAMES FOUND"
			if a.loading {
				message = "LOADING GAMES..."
			} else if a.catalogError {
				message = "SOURCE UNAVAILABLE"
			}
			label(r, 160, 162, message, [3]uint8{210, 223, 239})
			label(r, 120, 184, "X NEXT SOURCE  SELECT KEYBOARD", [3]uint8{155, 175, 195})
		}
		top := (a.selected / 8) * 8
		for i := top; i < len(a.games) && i < top+8; i++ {
			game := a.games[i]
			column, row := (i-top)%4, (i-top)/4
			x, y := int32(8+column*125), int32(48+row*151)
			highlight := [3]uint8{37, 51, 68}
			if i == a.selected {
				highlight = [3]uint8{70, 180, 226}
			}
			panel(r, x, y, 116, 147, highlight)
			panel(r, x+2, y+2, 112, 143, [3]uint8{24, 33, 48})
			panel(r, x+11, y+4, 94, 116, [3]uint8{42, 55, 75})
			if texture := a.coverTexture(game); texture != 0 {
				dest := sdlRect{x + 11, y + 4, 94, 116}
				sdl.RenderCopy(r, texture, nil, &dest)
			} else {
				label(r, x+26, y+55, "NO ART", [3]uint8{155, 175, 195})
			}
			badge := badgeLabel(game.System)
			badgeWidth := int32(len(badge)*8 + 8)
			panel(r, x+108-badgeWidth, y+6, badgeWidth, 15, [3]uint8{68, 77, 135})
			label(r, x+112-badgeWidth, y+9, badge, [3]uint8{242, 244, 255})
			label(r, x+6, y+126, shortTitle(game.Title, 13), [3]uint8{235, 239, 246})
		}
	}
	panel(r, 0, 351, 512, 33, [3]uint8{35, 52, 76})
	status := a.status
	if a.mode == "results" && !a.busy && !a.showStatus && len(a.games) > 0 {
		selected := a.games[a.selected]
		status = selected.Title + " [" + selected.System + "]"
	}
	if a.job != nil && (a.job.State == "queued" || a.job.State == "running") {
		status = a.job.Stage + ": " + a.job.Game.Title
		if a.job.Total > 0 {
			status = fmt.Sprintf("%s %d%%: %s", a.job.Stage, min(100, int(a.job.Done*100/a.job.Total)), a.job.Game.Title)
		} else if a.job.Done > 0 {
			status = fmt.Sprintf("%s %d MiB: %s", a.job.Stage, a.job.Done>>20, a.job.Game.Title)
		}
		panel(r, 10, 366, 492, 5, [3]uint8{19, 29, 44})
		if a.job.Total > 0 {
			width := int32(min(int64(492), a.job.Done*492/a.job.Total))
			if width > 0 {
				panel(r, 10, 366, width, 5, [3]uint8{70, 180, 226})
			}
		} else {
			offset := int32((a.job.Done >> 16) % 400)
			panel(r, 10+offset, 366, 92, 5, [3]uint8{70, 180, 226})
		}
	}
	label(r, 10, 356, status, [3]uint8{255, 215, 130})
	footer := "A INSTALL  L/R PAGE  X SOURCE  SELECT TYPE  START EXIT"
	if a.moreBusy {
		footer = "LOADING MORE...  L/R PAGE  SELECT TYPE  START EXIT"
	}
	if a.job != nil && (a.job.State == "queued" || a.job.State == "running") {
		footer = "L/R PAGE  X SOURCE  SELECT TYPE  START EXIT (DL RUNS)"
	}
	label(r, 10, 373, footer, [3]uint8{210, 223, 239})
	if path := os.Getenv("ROM_SEARCH_CAPTURE"); path != "" && !a.captured {
		a.captured = true
		shot := image.NewRGBA(image.Rect(0, 0, 1024, 768))
		// ABGR8888 stores RGBA bytes on this little-endian device.
		if sdl.RenderReadPixels(r, nil, 0x16762004, &shot.Pix[0], int32(shot.Stride)) == 0 {
			if file, e := os.Create(path); e == nil {
				if e = png.Encode(file, shot); e != nil {
					log.Printf("capture: %v", e)
				}
				file.Close()
			} else {
				log.Printf("capture: %v", e)
			}
		} else {
			log.Printf("capture: %s", sdl.GetError())
		}
	}
	sdl.RenderPresent(r)
}
func runUI() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	started := time.Now()
	if e := bindSDL(); e != nil {
		return e
	}
	log.Printf("startup: SDL bindings %s", time.Since(started))
	if sdl.SetHint("SDL_JOYSTICK_ALLOW_BACKGROUND_EVENTS", "1") == 0 {
		return fmt.Errorf("SDL controller background input hint was rejected")
	}
	if sdl.Init(0x2220) != 0 {
		return fmt.Errorf("SDL init: %s", sdl.GetError())
	}
	defer sdl.Quit()
	log.Printf("startup: SDL initialized %s", time.Since(started))
	window := sdl.CreateWindow("ROM Search", 0x2FFF0000, 0x2FFF0000, 1024, 768, 0x4)
	if window == 0 {
		return fmt.Errorf("SDL window: %s", sdl.GetError())
	}
	defer sdl.DestroyWindow(window)
	log.Printf("startup: window created %s", time.Since(started))
	renderer := sdl.CreateRenderer(window, -1, 0x2)
	if renderer == 0 {
		renderer = sdl.CreateRenderer(window, -1, 0x1)
	}
	if renderer == 0 {
		return fmt.Errorf("SDL renderer: %s", sdl.GetError())
	}
	defer sdl.DestroyRenderer(renderer)
	log.Printf("startup: renderer created %s", time.Since(started))
	sdl.RenderSetLogicalSize(renderer, 512, 384)
	sdl.GameControllerEventState(1)
	sdl.JoystickEventState(1)
	var controller uintptr
	if sdl.NumJoysticks() > 0 {
		controller = sdl.GameControllerOpen(0)
		if controller != 0 {
			defer sdl.GameControllerClose(controller)
		}
	}
	log.Printf("startup: controllers=%d, open=%v", sdl.NumJoysticks(), controller != 0)
	app := newApp()
	defer app.closeTextures()
	log.Printf("startup: app ready %s", time.Since(started))
	var event [64]byte
	dirty := true
	var axisDirection [2]int
	var axisRepeatAt [2]time.Time
	traceInput := os.Getenv("ROM_SEARCH_TRACE_INPUT") == "1"
	var lastPhysicalButton uint8
	lastButtonAt := time.Time{}
	handleEvent := func(event *[64]byte) {
		kind := binary.LittleEndian.Uint32(event[:4])
		action := ""
		if kind == 0x200 {
			dirty = true
		}
		if kind == 0x100 {
			action = "quit"
		}
		if kind == 0x300 {
			key := binary.LittleEndian.Uint32(event[20:24])
			switch key {
			case 1073741906:
				action = "up"
			case 1073741905:
				action = "down"
			case 1073741904:
				action = "left"
			case 1073741903:
				action = "right"
			case 13:
				action = "search"
			case 8:
				action = "back"
			case 27:
				action = "quit"
			case 120:
				action = "source"
			case 121:
				action = "edit"
			case 32:
				action = "accept"
			case 113:
				action = "pagePrev"
			case 101:
				action = "pageNext"
			}
		}
		if kind == 0x650 && event[12] < 2 {
			axis := int(event[12])
			value := int16(binary.LittleEndian.Uint16(event[16:18]))
			direction := 0
			if value < -17000 {
				direction = -1
			} else if value > 17000 {
				direction = 1
			}
			if direction != axisDirection[axis] {
				axisDirection[axis] = direction
				axisRepeatAt[axis] = time.Now().Add(350 * time.Millisecond)
				if direction != 0 {
					if axis == 0 && direction < 0 {
						action = "left"
					} else if axis == 0 {
						action = "right"
					} else if direction < 0 {
						action = "up"
					} else {
						action = "down"
					}
				}
			}
		}
		if kind == 0x651 || kind == 0x603 {
			physical := physicalButton(kind, event[12])
			if physical != 0 {
				if physical == lastPhysicalButton && time.Since(lastButtonAt) < 50*time.Millisecond {
					return
				}
				lastPhysicalButton, lastButtonAt = physical, time.Now()
			}
			action = buttonAction(kind, event[12])
			if physical != 0 && action != "" {
				log.Printf("button: kind=%#x code=%d action=%s", kind, event[12], action)
			}
		}
		if action != "" {
			app.press(action)
			dirty = true
			if traceInput {
				log.Printf("input: kind=%#x action=%s selected=%d mode=%s query=%q", kind, action, app.selected, app.mode, app.query)
			}
		} else if traceInput && (kind == 0x650 || kind == 0x651 || kind == 0x603 || kind == 0x300) {
			log.Printf("input: unmapped kind=%#x value=%d", kind, event[12])
		}
	}
	for app.running {
		hadEvent := false
		for sdl.PollEvent(&event[0]) != 0 {
			hadEvent = true
			handleEvent(&event)
		}
		if !app.running {
			break
		}
		if app.poll() {
			dirty = true
		}
		if dirty {
			renderStarted := time.Now()
			app.render(renderer)
			if traceInput {
				log.Printf("render: %s page=%d selected=%d mode=%s loading=%v catalogError=%v status=%q", time.Since(renderStarted), app.selected/8+1, app.selected, app.mode, app.loading, app.catalogError, app.status)
			}
			dirty = false
		}
		if !hadEvent {
			if app.preloadNearby(renderer) {
				dirty = true
			}
			for axis, direction := range axisDirection {
				if direction == 0 || time.Now().Before(axisRepeatAt[axis]) {
					continue
				}
				if axis == 0 {
					if direction < 0 {
						app.press("left")
					} else {
						app.press("right")
					}
				} else if direction < 0 {
					app.press("up")
				} else {
					app.press("down")
				}
				dirty = true
				axisRepeatAt[axis] = time.Now().Add(140 * time.Millisecond)
			}
			time.Sleep(16 * time.Millisecond)
		}
	}
	return nil
}
