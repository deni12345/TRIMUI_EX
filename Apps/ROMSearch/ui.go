package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ebitengine/purego"
)

var sdl struct {
	Init                 func(uint32) int32
	Quit                 func()
	CreateWindow         func(string, int32, int32, int32, int32, uint32) uintptr
	DestroyWindow        func(uintptr)
	CreateRenderer       func(uintptr, int32, uint32) uintptr
	DestroyRenderer      func(uintptr)
	RenderSetLogicalSize func(uintptr, int32, int32) int32
	SetRenderDrawColor   func(uintptr, uint8, uint8, uint8, uint8) int32
	RenderClear          func(uintptr) int32
	RenderPresent        func(uintptr)
	PollEvent            func(*byte) int32
	Delay                func(uint32)
	GetError             func() string
	NumJoysticks         func() int32
	GameControllerOpen   func(int32) uintptr
	GameControllerClose  func(uintptr)
	Box                  func(uintptr, int32, int32, int32, int32, uint8, uint8, uint8, uint8) int32
	String               func(uintptr, int32, int32, string, uint8, uint8, uint8, uint8) int32
}

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
	purego.RegisterLibFunc(&sdl.PollEvent, lib, "SDL_PollEvent")
	purego.RegisterLibFunc(&sdl.Delay, lib, "SDL_Delay")
	purego.RegisterLibFunc(&sdl.GetError, lib, "SDL_GetError")
	purego.RegisterLibFunc(&sdl.NumJoysticks, lib, "SDL_NumJoysticks")
	purego.RegisterLibFunc(&sdl.GameControllerOpen, lib, "SDL_GameControllerOpen")
	purego.RegisterLibFunc(&sdl.GameControllerClose, lib, "SDL_GameControllerClose")
	purego.RegisterLibFunc(&sdl.Box, gfx, "boxRGBA")
	purego.RegisterLibFunc(&sdl.String, gfx, "stringRGBA")
	return nil
}

var keyboard = []string{"ABCDEFGH", "IJKLMNOP", "QRSTUVWX", "YZ012345", "6789 -_."}

type result struct {
	kind        string
	games       []Game
	next        bool
	path        string
	err         error
	done, total int64
	message     string
}
type App struct {
	source           int
	system           int
	systems          []System
	query            string
	page             int
	games            []Game
	selected         int
	mode             string
	keyX, keyY       int
	busy             bool
	status           string
	messages         chan result
	romRoot, imgRoot string
	running          bool
	hasNext          bool
}

func newApp() *App {
	root := os.Getenv("ROM_SEARCH_ROOT")
	if root == "" {
		root = "/mnt/SDCARD/Roms"
	}
	img := os.Getenv("ROM_SEARCH_IMG_ROOT")
	if img == "" {
		img = "/mnt/SDCARD/Imgs"
	}
	a := &App{source: 2, systems: configuredSystems(root), romRoot: root, imgRoot: img, page: 1, mode: "results", messages: make(chan result, 100), running: true}
	if len(a.systems) == 0 {
		a.status = "No configured emulator ROM folders"
		return a
	}
	a.browse()
	return a
}
func (a *App) browse() {
	if a.busy || len(a.systems) == 0 {
		return
	}
	a.busy = true
	a.games = nil
	a.selected = 0
	a.status = "Loading " + sources[a.source] + " / " + a.systems[a.system].Folder + "..."
	source, system, page, query := sources[a.source], a.systems[a.system], a.page, a.query
	go func() {
		games, next, e := catalog(source, system, page, query)
		a.messages <- result{kind: "browse", games: games, next: next, err: e}
	}()
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
func (a *App) poll() {
	for {
		select {
		case r := <-a.messages:
			switch r.kind {
			case "browse":
				a.busy = false
				if r.err != nil {
					a.status = r.err.Error()
				} else {
					a.games = r.games
					a.hasNext = r.next
					a.status = fmt.Sprintf("%d games  |  page %d", len(r.games), a.page)
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
				if r.err != nil {
					a.status = r.err.Error()
				} else {
					a.status = "Installed " + filepath.Base(r.path)
				}
			}
		default:
			return
		}
	}
}
func (a *App) press(action string) {
	if action == "quit" {
		a.running = false
		return
	}
	if a.busy {
		return
	}
	switch action {
	case "source":
		a.source = (a.source + 1) % len(sources)
		a.query = ""
		a.page = 1
		a.mode = "results"
		a.browse()
	case "systemPrev", "systemNext":
		if len(a.systems) == 0 {
			return
		}
		step := 1
		if action == "systemPrev" {
			step = -1
		}
		a.system = (a.system + len(a.systems) + step) % len(a.systems)
		a.query = ""
		a.page = 1
		a.mode = "results"
		a.browse()
	case "edit":
		if a.mode == "keyboard" {
			a.mode = "results"
		} else {
			a.mode = "keyboard"
			a.status = "Type name, START search; B deletes"
		}
	case "search":
		if len(strings.TrimSpace(a.query)) < 3 {
			a.mode = "keyboard"
			a.status = "Enter at least 3 letters"
			return
		}
		a.page = 1
		a.mode = "results"
		a.browse()
	case "back":
		if a.mode == "keyboard" {
			if len(a.query) > 0 {
				a.query = a.query[:len(a.query)-1]
			} else {
				a.mode = "results"
			}
		} else {
			a.query = ""
			a.page = 1
			a.browse()
		}
	case "accept":
		if a.mode == "keyboard" {
			if len(a.query) < 100 {
				a.query += strings.ToLower(string(keyboard[a.keyY][a.keyX]))
			}
		} else {
			a.install()
		}
	case "up", "down":
		step := 1
		if action == "up" {
			step = -1
		}
		if a.mode == "keyboard" {
			a.keyY = max(0, min(len(keyboard)-1, a.keyY+step))
			a.keyX = min(a.keyX, len(keyboard[a.keyY])-1)
		} else {
			a.selected = max(0, min(len(a.games)-1, a.selected+step))
		}
	case "left", "right":
		step := 1
		if action == "left" {
			step = -1
		}
		if a.mode == "keyboard" {
			a.keyX = max(0, min(len(keyboard[a.keyY])-1, a.keyX+step))
		}
		if a.mode == "results" {
			next := a.page + step
			if next >= 1 && (step < 0 || a.hasNext) {
				a.page = next
				a.browse()
			}
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
func (a *App) render(r uintptr) {
	sdl.SetRenderDrawColor(r, 14, 19, 31, 255)
	sdl.RenderClear(r)
	panel(r, 0, 0, 512, 38, [3]uint8{35, 52, 76})
	label(r, 12, 10, "ROM SEARCH", [3]uint8{120, 214, 255})
	label(r, 232, 10, "SOURCE: "+strings.ToUpper(sources[a.source]), [3]uint8{230, 235, 245})
	system := "NONE"
	if len(a.systems) > 0 {
		system = a.systems[a.system].Folder
	}
	label(r, 12, 46, "EMULATOR: "+system+"  < L / R >", [3]uint8{230, 235, 245})
	label(r, 12, 64, "SEARCH: "+a.query, [3]uint8{230, 235, 245})
	if a.mode == "keyboard" {
		label(r, 12, 85, "A type  START search  B erase  Y list", [3]uint8{120, 214, 255})
		for y, row := range keyboard {
			for x := range row {
				px, py := int32(40+x*54), int32(115+y*35)
				color := [3]uint8{42, 55, 75}
				if x == a.keyX && y == a.keyY {
					color = [3]uint8{44, 110, 152}
				}
				panel(r, px, py, 42, 28, color)
				label(r, px+17, py+10, string(row[x]), [3]uint8{230, 235, 245})
			}
		}
	} else {
		label(r, 12, 87, "A install   Y search   X source   <- page ->", [3]uint8{120, 214, 255})
		top := max(0, a.selected-10)
		for i := top; i < len(a.games) && i < top+10; i++ {
			y := int32(108 + (i-top)*21)
			if i == a.selected {
				panel(r, 8, y-3, 496, 19, [3]uint8{44, 110, 152})
			}
			label(r, 12, y, a.games[i].Title, [3]uint8{230, 235, 245})
		}
		label(r, 12, 320, fmt.Sprintf("Page %d  |  %d shown", a.page, len(a.games)), [3]uint8{155, 175, 195})
	}
	panel(r, 0, 340, 512, 44, [3]uint8{35, 52, 76})
	label(r, 12, 346, a.status, [3]uint8{255, 215, 130})
	label(r, 12, 365, "SELECT exit  |  Only download files you may use", [3]uint8{230, 235, 245})
	sdl.RenderPresent(r)
}
func runUI() error {
	if e := bindSDL(); e != nil {
		return e
	}
	if sdl.Init(0x2220) != 0 {
		return fmt.Errorf("SDL init: %s", sdl.GetError())
	}
	defer sdl.Quit()
	window := sdl.CreateWindow("ROM Search", 0x2FFF0000, 0x2FFF0000, 1024, 768, 0x4)
	if window == 0 {
		return fmt.Errorf("SDL window: %s", sdl.GetError())
	}
	defer sdl.DestroyWindow(window)
	renderer := sdl.CreateRenderer(window, -1, 0x2)
	if renderer == 0 {
		renderer = sdl.CreateRenderer(window, -1, 0x1)
	}
	if renderer == 0 {
		return fmt.Errorf("SDL renderer: %s", sdl.GetError())
	}
	defer sdl.DestroyRenderer(renderer)
	sdl.RenderSetLogicalSize(renderer, 512, 384)
	var controller uintptr
	if sdl.NumJoysticks() > 0 {
		controller = sdl.GameControllerOpen(0)
		if controller != 0 {
			defer sdl.GameControllerClose(controller)
		}
	}
	app := newApp()
	var event [64]byte
	for app.running {
		app.poll()
		for sdl.PollEvent(&event[0]) != 0 {
			kind := binary.LittleEndian.Uint32(event[:4])
			action := ""
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
					action = "systemPrev"
				case 101:
					action = "systemNext"
				}
			}
			if kind == 0x651 {
				switch event[12] {
				case 0:
					action = "back"
				case 1:
					action = "accept"
				case 2:
					action = "source"
				case 3:
					action = "edit"
				case 6:
					action = "quit"
				case 7:
					action = "search"
				case 9:
					action = "systemPrev"
				case 10:
					action = "systemNext"
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
			}
		}
		app.render(renderer)
		sdl.Delay(30)
	}
	return nil
}
