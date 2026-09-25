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
	WaitEventTimeout         func(*byte, int32) int32
	GetError                 func() string
	NumJoysticks             func() int32
	GameControllerOpen       func(int32) uintptr
	GameControllerClose      func(uintptr)
	GameControllerEventState func(int32) int32
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
	purego.RegisterLibFunc(&sdl.WaitEventTimeout, lib, "SDL_WaitEventTimeout")
	purego.RegisterLibFunc(&sdl.GetError, lib, "SDL_GetError")
	purego.RegisterLibFunc(&sdl.NumJoysticks, lib, "SDL_NumJoysticks")
	purego.RegisterLibFunc(&sdl.GameControllerOpen, lib, "SDL_GameControllerOpen")
	purego.RegisterLibFunc(&sdl.GameControllerClose, lib, "SDL_GameControllerClose")
	purego.RegisterLibFunc(&sdl.GameControllerEventState, lib, "SDL_GameControllerEventState")
	purego.RegisterLibFunc(&sdl.Box, gfx, "boxRGBA")
	purego.RegisterLibFunc(&sdl.String, gfx, "stringRGBA")
	imageLib, e := purego.Dlopen("libSDL2_image-2.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if e != nil {
		return e
	}
	purego.RegisterLibFunc(&sdl.LoadTexture, imageLib, "IMG_LoadTexture")
	return nil
}

var keyboard = []string{"ABCDEFGH", "IJKLMNOP", "QRSTUVWX", "YZ012345", "6789 -_."}

type result struct {
	kind        string
	id          int
	key         string
	games       []Game
	path        string
	err         error
	done, total int64
	message     string
}
type catalogPage struct {
	games []Game
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
	busy             bool
	loading          bool
	browseID         int
	browseCancel     context.CancelFunc
	status           string
	showStatus       bool
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
	a := &App{source: 2, displaySource: 2, systems: configuredSystems(root), romRoot: root, imgRoot: img, mode: "results", messages: make(chan result, 100), cache: make(map[string]catalogPage), coverPending: make(map[string]bool), coverFailed: make(map[string]bool), coverSlots: make(chan struct{}, 2), textures: make(map[string]uintptr), preloadPage: -1, running: true}
	log.Printf("startup: emulator scan %s, %d systems", time.Since(started), len(a.systems))
	if len(a.systems) == 0 {
		a.status = "No configured emulator ROM folders"
		return a
	}
	a.games = featuredGames(a.systems)
	a.cache["RomsGames|"] = catalogPage{games: a.games}
	a.status = fmt.Sprintf("%d featured games  |  Y search", len(a.games))
	return a
}
func (a *App) browse() {
	if a.busy || len(a.systems) == 0 {
		return
	}
	source, query := sources[a.source], a.query
	key := source + "|" + query
	if cached, found := a.cache[key]; found {
		a.games = cached.games
		a.displaySource = a.source
		a.showStatus = false
		a.preloadPage = -1
		a.selected = 0
		a.mode = "results"
		a.status = fmt.Sprintf("%d games  |  Y search", len(cached.games))
		return
	}
	a.busy = true
	a.loading = true
	a.browseID++
	a.selected = 0
	a.status = "Loading games... X switches, B cancels"
	a.showStatus = false
	ctx, cancel := context.WithCancel(context.Background())
	a.browseCancel = cancel
	id := a.browseID
	go func() {
		started := time.Now()
		games, e := catalogMixed(ctx, source, query, a.systems)
		log.Printf("catalog: %s query %q in %s, %d games, error=%v", source, query, time.Since(started), len(games), e)
		a.messages <- result{kind: "browse", id: id, key: key, games: games, err: e}
	}()
}
func (a *App) cancelLoading() {
	if a.browseCancel != nil {
		a.browseCancel()
		a.browseCancel = nil
	}
	a.browseID++
	a.busy = false
	a.loading = false
	a.mode = "results"
	a.status = "Loading cancelled"
	a.showStatus = true
}
func (a *App) install() {
	if a.busy || len(a.games) == 0 {
		return
	}
	game := a.games[a.selected]
	a.busy = true
	a.status = "Downloading " + game.Title + "..."
	go func() {
		progress := func(done, total int64) {
			select {
			case a.messages <- result{kind: "progress", done: done, total: total}:
			default:
			}
		}
		status := func(s string) { a.messages <- result{kind: "status", message: s} }
		p, e := install(game, a.romRoot, a.imgRoot, progress, status)
		a.messages <- result{kind: "installed", path: p, err: e}
	}()
}
func (a *App) poll() bool {
	changed := false
	for {
		select {
		case r := <-a.messages:
			changed = true
			switch r.kind {
			case "browse":
				if r.id != a.browseID {
					continue
				}
				a.busy = false
				a.loading = false
				a.browseCancel = nil
				if r.err != nil {
					a.status = sources[a.source] + ": " + r.err.Error()
					a.showStatus = true
				} else {
					a.cache[r.key] = catalogPage{games: r.games}
					a.mode = "results"
					a.games = r.games
					a.displaySource = a.source
					a.showStatus = false
					a.preloadPage = -1
					a.selected = 0
					a.status = fmt.Sprintf("%d games  |  Y search", len(r.games))
				}
			case "cover":
				delete(a.coverPending, r.key)
				if r.err != nil {
					a.coverFailed[r.key] = true
					log.Printf("cover: %s: %v", r.key, r.err)
				} else {
					a.preloadPage = -1
				}
			case "progress":
				if r.total > 0 {
					a.status = fmt.Sprintf("Downloading %d / %d MiB", r.done>>20, r.total>>20)
				} else {
					a.status = fmt.Sprintf("Downloading %d MiB", r.done>>20)
				}
			case "status":
				a.status = r.message
			case "installed":
				a.busy = false
				log.Printf("install: path=%q error=%v", r.path, r.err)
				if r.err != nil {
					a.status = r.err.Error()
				} else {
					a.status = "Installed " + filepath.Base(r.path)
				}
			}
		default:
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
func (a *App) press(action string) {
	if action == "quit" {
		if a.loading {
			a.cancelLoading()
		}
		a.running = false
		return
	}
	if a.busy && !a.loading {
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
			a.status = "Type a name, then press START"
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
			if len(a.query) < 100 {
				a.query += strings.ToLower(string(keyboard[a.keyY][a.keyX]))
			}
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
			a.keyY = max(0, min(len(keyboard)-1, a.keyY+step))
			a.keyX = min(a.keyX, len(keyboard[a.keyY])-1)
		} else if len(a.games) > 0 {
			a.selected = max(0, min(len(a.games)-1, a.selected+step))
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
			a.keyX = max(0, min(len(keyboard[a.keyY])-1, a.keyX+step))
		} else if len(a.games) > 0 {
			a.selected = max(0, min(len(a.games)-1, a.selected+step))
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
func (a *App) render(r uintptr) {
	sdl.SetRenderDrawColor(r, 14, 19, 31, 255)
	sdl.RenderClear(r)
	panel(r, 0, 0, 512, 43, [3]uint8{35, 52, 76})
	label(r, 10, 7, "ROM SEARCH", [3]uint8{120, 214, 255})
	label(r, 224, 7, strings.ToUpper(sources[a.source]), [3]uint8{230, 235, 245})
	totalPages := max(1, (len(a.games)+7)/8)
	currentPage := a.selected/8 + 1
	label(r, 376, 7, fmt.Sprintf("%d GAMES %d/%d", len(a.games), currentPage, totalPages), [3]uint8{210, 223, 239})
	searchBorder := [3]uint8{75, 94, 118}
	if a.mode == "keyboard" {
		searchBorder = [3]uint8{70, 180, 226}
	}
	panel(r, 8, 23, 496, 19, searchBorder)
	panel(r, 10, 25, 492, 15, [3]uint8{19, 29, 44})
	searchText := a.query
	if searchText == "" {
		searchText = "Y TO ENTER A GAME NAME"
	} else if a.mode == "keyboard" {
		searchText += "_"
	}
	if len(searchText) > 46 {
		searchText = searchText[len(searchText)-46:]
	}
	label(r, 15, 29, "SEARCH: "+searchText, [3]uint8{210, 223, 239})
	if a.mode == "keyboard" {
		label(r, 10, 56, "A TYPE  START SEARCH  B ERASE  Y BACK", [3]uint8{210, 223, 239})
		for y, row := range keyboard {
			for x := range row {
				px, py := int32(40+x*54), int32(100+y*35)
				color := [3]uint8{42, 55, 75}
				if x == a.keyX && y == a.keyY {
					color = [3]uint8{44, 110, 152}
				}
				panel(r, px, py, 42, 28, color)
				label(r, px+17, py+10, string(row[x]), [3]uint8{230, 235, 245})
			}
		}
	} else {
		a.ensureCovers()
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
	label(r, 10, 356, status, [3]uint8{255, 215, 130})
	label(r, 10, 373, "A INSTALL  L/R PAGE  X SOURCE  Y SEARCH  SELECT EXIT", [3]uint8{210, 223, 239})
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
		if kind == 0x651 {
			switch event[12] {
			case 0:
				action = "accept"
			case 1:
				action = "back"
			case 2:
				action = "source"
			case 3:
				action = "edit"
			case 4:
				action = "quit"
			case 6:
				action = "search"
			case 9:
				action = "pagePrev"
			case 10:
				action = "pageNext"
			case 11:
				action = "up"
			case 12:
				action = "down"
			case 13:
				action = "left"
			case 14:
				action = "right"
			}
		}
		if action != "" {
			app.press(action)
			dirty = true
			if traceInput {
				log.Printf("input: kind=%#x action=%s selected=%d mode=%s query=%q", kind, action, app.selected, app.mode, app.query)
			}
		} else if traceInput && (kind == 0x650 || kind == 0x651 || kind == 0x300) {
			log.Printf("input: unmapped kind=%#x value=%d", kind, event[12])
		}
	}
	for app.running {
		if app.poll() {
			dirty = true
		}
		if dirty {
			renderStarted := time.Now()
			app.render(renderer)
			if traceInput {
				log.Printf("render: %s page=%d selected=%d mode=%s", time.Since(renderStarted), app.selected/8+1, app.selected, app.mode)
			}
			dirty = false
		}
		if sdl.WaitEventTimeout(&event[0], 30) != 0 {
			handleEvent(&event)
			for sdl.PollEvent(&event[0]) != 0 {
				handleEvent(&event)
			}
		} else {
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
		}
	}
	return nil
}
