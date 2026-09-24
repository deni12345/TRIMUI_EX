#!/usr/bin/env python3
"""Small SDL controller UI for searching and installing ROM archives."""
import ctypes as C
import os
from pathlib import Path
import queue
import threading

from rom_sources import download, installable, save_thumbnail, search_coolrom, search_romsfun


ROOT = Path(os.environ.get("ROM_SEARCH_ROOT", "/mnt/SDCARD/Roms"))
IMG_ROOT = Path(os.environ.get("ROM_SEARCH_IMG_ROOT", "/mnt/SDCARD/Imgs"))
KEYS = ["ABCDEFGH", "IJKLMNOP", "QRSTUVWX", "YZ012345", "6789 -_."]


def lib(*names):
    for name in names:
        try:
            return C.CDLL(name)
        except OSError:
            pass
    raise RuntimeError("SDL library missing: " + names[0])


SDL = lib("libSDL2-2.0.so.0", "libSDL2.so")
GFX = lib("libSDL2_gfx-1.0.so.0", "libSDL2_gfx.so")


def bind(dll, name, result, *args):
    fn = getattr(dll, name)
    fn.restype = result
    fn.argtypes = args
    return fn


P = C.c_void_p
I = C.c_int
U = C.c_uint
B = C.c_uint8
SDL_Init = bind(SDL, "SDL_Init", I, U)
SDL_Quit = bind(SDL, "SDL_Quit", None)
SDL_CreateWindow = bind(SDL, "SDL_CreateWindow", P, C.c_char_p, I, I, I, I, U)
SDL_DestroyWindow = bind(SDL, "SDL_DestroyWindow", None, P)
SDL_CreateRenderer = bind(SDL, "SDL_CreateRenderer", P, P, I, U)
SDL_DestroyRenderer = bind(SDL, "SDL_DestroyRenderer", None, P)
SDL_RenderSetLogicalSize = bind(SDL, "SDL_RenderSetLogicalSize", I, P, I, I)
SDL_SetRenderDrawColor = bind(SDL, "SDL_SetRenderDrawColor", I, P, B, B, B, B)
SDL_RenderClear = bind(SDL, "SDL_RenderClear", I, P)
SDL_RenderPresent = bind(SDL, "SDL_RenderPresent", None, P)
SDL_PollEvent = bind(SDL, "SDL_PollEvent", I, P)
SDL_Delay = bind(SDL, "SDL_Delay", None, U)
SDL_GetError = bind(SDL, "SDL_GetError", C.c_char_p)
SDL_NumJoysticks = bind(SDL, "SDL_NumJoysticks", I)
SDL_GameControllerOpen = bind(SDL, "SDL_GameControllerOpen", P, I)
SDL_GameControllerClose = bind(SDL, "SDL_GameControllerClose", None, P)
box = bind(GFX, "boxRGBA", I, P, I, I, I, I, B, B, B, B)
string = bind(GFX, "stringRGBA", I, P, I, I, C.c_char_p, B, B, B, B)


def draw_text(renderer, x, y, value, color=(230, 235, 245)):
    # SDL_gfx's built-in font is ASCII. Replace unsupported characters cleanly.
    encoded = str(value).encode("ascii", "replace")
    string(renderer, x, y, encoded[:62], *color, 255)


def panel(renderer, x, y, w, h, color):
    box(renderer, x, y, x + w, y + h, *color, 255)


class App:
    def __init__(self):
        self.source = "CoolROM"
        self.query = ""
        self.mode = "keyboard"
        self.key_x = 0
        self.key_y = 0
        self.games = []
        self.selected = 0
        self.status = "Enter a game name and press START to search"
        self.busy = False
        self.messages = queue.Queue()
        self.running = True

    def work(self, fn):
        if self.busy:
            return
        self.busy = True

        def task():
            try:
                result = fn()
                self.messages.put(("done", result))
            except Exception as exc:
                self.messages.put(("error", str(exc)))

        threading.Thread(target=task, daemon=True).start()

    def search(self):
        if self.busy:
            return
        if len(self.query.strip()) < 3:
            self.status = "Enter at least 3 letters to search"
            return
        self.games = []
        self.status = "Searching %s..." % self.source
        adapter = search_coolrom if self.source == "CoolROM" else search_romsfun
        self.work(lambda: ("search", adapter(self.query.strip())))

    def install(self):
        if not self.games:
            return
        game = self.games[self.selected]
        self.status = "Downloading %s..." % game.title

        def progress(done, total):
            self.messages.put(("progress", (done, total)))

        def action():
            path = download(game, ROOT, progress,
                            lambda message: self.messages.put(("status", message)))
            self.messages.put(("status", "Saving thumbnail..."))
            try:
                artwork = save_thumbnail(game, path, IMG_ROOT)
                return ("install", path, artwork, None)
            except Exception as exc:
                return ("install", path, None, str(exc))

        self.work(action)

    def poll(self):
        while True:
            try:
                kind, value = self.messages.get_nowait()
            except queue.Empty:
                return
            if kind == "progress":
                done, total = value
                self.status = "Downloading: %d / %d MiB" % (done // 1048576, total // 1048576)
            elif kind == "status":
                self.status = value
            elif kind == "error":
                self.busy = False
                self.status = value
            else:
                self.busy = False
                action, *details = value
                if action == "search":
                    result = details[0]
                    self.games = installable(result, ROOT)
                    self.selected = 0
                    self.mode = "results"
                    self.status = "%d games for installed emulators" % len(self.games) if self.games else "No games for installed emulators"
                else:
                    path, artwork, error = details
                    self.status = ("ROM and thumbnail saved: %s" % path.name
                                   if artwork else "ROM saved; thumbnail failed: %s" % error)

    def press(self, name):
        if name == "quit":
            self.running = False
        elif name == "source" and not self.busy:
            self.source = "RomsFun" if self.source == "CoolROM" else "CoolROM"
            self.games = []
            self.mode = "keyboard"
            self.status = "Source: %s" % self.source
        elif name == "search":
            self.search()
        elif name == "edit":
            self.mode = "keyboard"
        elif name == "back":
            if self.mode == "keyboard":
                self.query = self.query[:-1]
            else:
                self.mode = "keyboard"
        elif name in ("up", "down", "left", "right"):
            step = -1 if name in ("up", "left") else 1
            if self.mode == "results":
                self.selected = max(0, min(len(self.games) - 1, self.selected + step))
            elif name in ("up", "down"):
                self.key_y = max(0, min(len(KEYS) - 1, self.key_y + step))
                self.key_x = min(self.key_x, len(KEYS[self.key_y]) - 1)
            else:
                self.key_x = max(0, min(len(KEYS[self.key_y]) - 1, self.key_x + step))
        elif name == "accept" and not self.busy:
            if self.mode == "results":
                self.install()
            elif len(self.query) < 100:
                self.query += KEYS[self.key_y][self.key_x].lower()

    def render(self, renderer):
        SDL_SetRenderDrawColor(renderer, 14, 19, 31, 255)
        SDL_RenderClear(renderer)
        panel(renderer, 0, 0, 512, 36, (35, 52, 76))
        draw_text(renderer, 12, 10, "ROM SEARCH", (120, 214, 255))
        draw_text(renderer, 232, 10, "SOURCE: " + self.source.upper())
        draw_text(renderer, 12, 45, "SEARCH: " + self.query[-48:])
        panel(renderer, 12, 60, 488, 1, (80, 104, 130))
        if self.mode == "keyboard":
            draw_text(renderer, 12, 74, "Choose letters with D-pad, A to type")
            for y, row in enumerate(KEYS):
                for x, char in enumerate(row):
                    px, py = 40 + x * 54, 112 + y * 35
                    panel(renderer, px, py, 42, 28,
                          (44, 110, 152) if (x, y) == (self.key_x, self.key_y)
                          else (42, 55, 75))
                    draw_text(renderer, px + 17, py + 10, char)
            draw_text(renderer, 12, 307, "START search   B erase   X source")
        else:
            draw_text(renderer, 12, 69, "A download selected game   Y edit search")
            top = max(0, self.selected - 10)
            for i, game in enumerate(self.games[top:top + 11], start=top):
                y = 88 + (i - top) * 21
                if i == self.selected:
                    panel(renderer, 8, y - 3, 496, 19, (44, 110, 152))
                draw_text(renderer, 12, y, "%s  [%s]" % (game.title[:45], game.system))
        panel(renderer, 0, 338, 512, 46, (35, 52, 76))
        draw_text(renderer, 12, 346, self.status[:61], (255, 215, 130))
        draw_text(renderer, 12, 365, "SELECT exit  |  Only download files you may use")
        SDL_RenderPresent(renderer)


def main():
    if SDL_Init(0x20 | 0x200 | 0x2000) != 0:
        raise RuntimeError("SDL initialization failed: " + SDL_GetError().decode("utf-8", "replace"))
    window = SDL_CreateWindow(b"ROM Search", 0x2FFF0000, 0x2FFF0000, 1024, 768, 0x4)
    if not window:
        raise RuntimeError("SDL window failed: " + SDL_GetError().decode("utf-8", "replace"))
    renderer = SDL_CreateRenderer(window, -1, 0x2)
    if not renderer:
        renderer = SDL_CreateRenderer(window, -1, 0x1)
    if not renderer:
        raise RuntimeError("SDL renderer failed: " + SDL_GetError().decode("utf-8", "replace"))
    SDL_RenderSetLogicalSize(renderer, 512, 384)
    controller = SDL_GameControllerOpen(0) if SDL_NumJoysticks() else None
    app = App()
    event = C.create_string_buffer(64)
    try:
        while app.running:
            app.poll()
            while SDL_PollEvent(event):
                event_type = C.c_uint32.from_buffer(event, 0).value
                if event_type == 0x100:
                    app.press("quit")
                elif event_type == 0x300:
                    key = C.c_int32.from_buffer(event, 20).value
                    action = {1073741906: "up", 1073741905: "down",
                              1073741904: "left", 1073741903: "right",
                              13: "search", 8: "back", 27: "quit",
                              120: "source", 121: "edit", 32: "accept"}.get(key)
                    if action:
                        app.press(action)
                elif event_type == 0x651:
                    button = C.c_uint8.from_buffer(event, 12).value
                    # TG4040's physical A/B and X/Y follow its Nintendo labels.
                    action = {0: "back", 1: "accept", 2: "source", 3: "edit",
                              6: "quit", 7: "search",
                              11: "up", 12: "down", 13: "left", 14: "right"}.get(button)
                    if action:
                        app.press(action)
            app.render(renderer)
            SDL_Delay(30)
    finally:
        if controller:
            SDL_GameControllerClose(controller)
        SDL_DestroyRenderer(renderer)
        SDL_DestroyWindow(window)
        SDL_Quit()


if __name__ == "__main__":
    main()
