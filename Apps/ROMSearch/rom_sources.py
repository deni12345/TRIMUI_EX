"""Public-page adapters and conservative ROM file installation."""
from dataclasses import dataclass
from functools import lru_cache
from html.parser import HTMLParser
from pathlib import Path, PurePosixPath
import html
import json
import os
import re
import shutil
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import zipfile


USER_AGENT = "TrimUI-ROM-Search/0.1"
MAX_PAGE = 2_000_000
MAX_ROM = 8 * 1024 ** 3
MAX_ART = 4 * 1024 ** 2
ALLOWED_EXT = {".zip", ".7z", ".rar", ".iso", ".chd", ".cue", ".bin",
               ".gba", ".gb", ".gbc", ".nes", ".sfc", ".smc", ".md",
               ".gen", ".gg", ".sms", ".n64", ".z64", ".v64", ".cso",
               ".pbp", ".nds", ".d64", ".adf"}
SYSTEM_FOLDERS = {
    "genesis": "MD", "mastersystem": "MS", "gamegear": "GG",
    "psx": "PS", "psp": "PSP", "saturn": "SS", "dc": "DC",
    "mame": "MAME", "neogeo": "NEOGEO", "cps1": "CPS1", "cps2": "CPS2",
    "atari2600": "ATARI2600", "atari5200": "ATARI5200",
    "atari7800": "ATARI7800", "atarilynx": "LYNX",
    "neogeopocket": "NGP", "segacd": "SEGACD", "c64": "C64",
    "nes": "FC", "snes": "SFC", "gba": "GBA", "gb": "GB",
    "gbc": "GBC", "n64": "N64", "nds": "NDS",
    "playstation": "PS", "playstation-portable": "PSP",
    "sega-genesis": "MD", "sega-saturn": "SS", "dreamcast": "DC",
    "nintendo-64": "N64", "nintendo-ds": "NDS",
    "game-boy": "GB", "game-boy-color": "GBC", "game-boy-advance": "GBA",
    "super-nintendo": "SFC",
}

# Files the installed emulators can open after an archive has been unpacked.
# Arcade sets are an exception: their ZIP filenames and ROM contents must stay intact.
PLAYABLE_EXT = {
    "MD": {".bin", ".gen", ".md", ".smd", ".32x"},
    "MS": {".sms", ".rom", ".gg", ".sg"}, "GG": {".gg"},
    "PS": {".bin", ".cue", ".img", ".mdf", ".pbp", ".toc", ".cbn", ".m3u", ".chd"},
    "PSP": {".iso", ".cso", ".pbp", ".chd"},
    "SS": {".cue", ".iso", ".chd", ".bin", ".img", ".mds", ".ccd"},
    "DC": {".gdi", ".cdi", ".chd", ".cue"},
    "MAME": {".zip"}, "NEOGEO": {".zip"},
    "CPS1": {".zip"}, "CPS2": {".zip"},
    "ATARI2600": {".a26", ".bin", ".rom"},
    "ATARI5200": {".a52", ".bin", ".rom"},
    "ATARI7800": {".a78", ".bin", ".rom"},
    "LYNX": {".lnx", ".lyx"}, "NGP": {".ngp", ".ngc"},
    "SEGACD": {".cue", ".iso", ".chd", ".bin", ".img"},
    "C64": {".d64", ".t64", ".tap", ".crt", ".prg", ".p00"},
    "FC": {".nes", ".fds", ".unf", ".unif"},
    "SFC": {".sfc", ".smc", ".fig", ".gd3", ".gd7", ".dx2", ".bsx", ".swc"},
    "GBA": {".gba", ".agb", ".gbz"},
    "GB": {".gb", ".gbc"}, "GBC": {".gbc", ".gb"},
    "N64": {".n64", ".z64", ".v64"}, "NDS": {".nds"},
}
ARCHIVE_EXT = {".zip", ".7z", ".rar"}
ARCADE_FOLDERS = {"MAME", "NEOGEO", "CPS1", "CPS2"}
DISC_FOLDERS = {"PS", "SS", "DC", "SEGACD"}
DISC_DESCRIPTORS = {".cue", ".gdi", ".m3u", ".mds", ".ccd"}
DISC_COMPANIONS = {".bin", ".img", ".iso", ".raw", ".wav", ".ape", ".flac", ".mp3", ".chd", ".mdf", ".sub"}
ALLOWED_EXT.update(ext for extensions in PLAYABLE_EXT.values() for ext in extensions)


class SourceError(Exception):
    pass


@dataclass(frozen=True)
class Game:
    source: str
    system: str
    title: str
    url: str


def request(url, *, referer=None):
    headers = {"User-Agent": USER_AGENT, "Accept": "text/html,application/octet-stream;q=0.9,*/*;q=0.8"}
    if referer:
        headers["Referer"] = referer
    return urllib.request.Request(url, headers=headers)


def read_page(url):
    try:
        with urllib.request.urlopen(request(url), timeout=20) as response:
            data = response.read(MAX_PAGE + 1)
            if len(data) > MAX_PAGE:
                raise SourceError("Page is too large")
            return data.decode("utf-8", "replace")
    except urllib.error.HTTPError as exc:
        if exc.code in (403, 429):
            raise SourceError("Site blocked automated access (HTTP %d)" % exc.code) from exc
        raise SourceError("HTTP %d from %s" % (exc.code, urllib.parse.urlsplit(url).hostname)) from exc
    except (urllib.error.URLError, TimeoutError) as exc:
        raise SourceError("Network error: %s" % exc) from exc


class LinkParser(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.links = []
        self.current = None

    def handle_starttag(self, tag, attrs):
        if tag == "a":
            self.current = [dict(attrs).get("href", ""), []]

    def handle_data(self, data):
        if self.current is not None:
            self.current[1].append(data)

    def handle_endtag(self, tag):
        if tag == "a" and self.current is not None:
            self.links.append((self.current[0], "".join(self.current[1]).strip()))
            self.current = None


def search_coolrom(query, system=""):
    params = {"q": query[:100]}
    if system:
        params["system"] = system
    page = read_page("https://coolrom.com/search?" + urllib.parse.urlencode(params))
    parser = LinkParser()
    # Search matches appear before the sidebar's unrelated popular titles.
    page = page.split("Top 25 Downloaded ROMs", 1)[0]
    parser.feed(page)
    games = []
    seen = set()
    for href, title in parser.links:
        match = re.fullmatch(r"/roms/([a-z0-9]+)/\d+/[^/]+\.php", href)
        if match and href not in seen:
            seen.add(href)
            games.append(Game("CoolROM", match.group(1), title,
                              urllib.parse.urljoin("https://coolrom.com", href)))
    return games


def search_romsfun(query, system=""):
    # The site currently challenges simple clients. Keep the adapter explicit so
    # the UI reports the real condition instead of inventing search results.
    page = read_page("https://romsfun.com/?s=" + urllib.parse.quote(query))
    if "cf-chl" in page or "Just a moment" in page:
        raise SourceError("RomsFun requires an interactive browser challenge")
    parser = LinkParser()
    parser.feed(page)
    games = []
    seen = set()
    for href, title in parser.links:
        match = re.fullmatch(r"https://romsfun\.com/roms/([^/]+)/[^/]+\.html", href)
        if match and title and href not in seen:
            seen.add(href)
            games.append(Game("RomsFun", match.group(1), title, href))
    return games


def coolrom_download_url(game):
    if game.source != "CoolROM" or not game.url.startswith("https://coolrom.com/roms/"):
        raise SourceError("Unsupported game page")
    page = read_page(game.url + "?v=" + str(time.time_ns()))
    links = re.findall(r'https://dl\.coolrom\.com/roms/[^"\s<>]+', page)
    if not links:
        raise SourceError("No direct download link on game page")
    return html.unescape(links[0]).replace("\\/", "/")


def romsfun_download_url(game):
    if game.source != "RomsFun" or not game.url.startswith("https://romsfun.com/roms/"):
        raise SourceError("Unsupported game page")
    page = read_page(game.url)
    match = re.search(r'href="(https://romsfun\.com/download/[^"/]+)"', page)
    if not match:
        raise SourceError("No download page found")
    page = read_page(match.group(1) + "/1?v=" + str(time.time_ns()))
    match = re.search(r'<a\s+href="([^"]+)"\s+id="download-link"', page)
    if not match:
        raise SourceError("No direct RomsFun file link found")
    return html.unescape(match.group(1))


@lru_cache(maxsize=8)
def configured_rom_folders(sd_root):
    """Read the actual emulator ROM paths, including PPSSPP's PSP folder."""
    emu_root = sd_root / "Emus"
    if not emu_root.is_dir():
        return None
    rom_root = (sd_root / "Roms").resolve()
    folders = set()
    for config in emu_root.glob("*/config.json"):
        try:
            data = json.loads(config.read_text(encoding="utf-8"))
            path = (config.parent / data["rompath"]).resolve()
            if path.parent == rom_root:
                folders.add(path.name)
        except (OSError, KeyError, ValueError, TypeError):
            continue
    return frozenset(folders)


def destination(rom_root, game):
    folder = SYSTEM_FOLDERS.get(game.system)
    if not folder or folder not in PLAYABLE_EXT:
        raise SourceError("No emulator folder mapped for %s" % game.system)
    path = Path(rom_root) / folder
    if not path.is_dir():
        raise SourceError("Emulator folder missing: %s" % path)
    configured = configured_rom_folders(Path(rom_root).parent.resolve())
    if configured is not None and folder not in configured:
        raise SourceError("No installed emulator uses %s" % path)
    return path


def installable(games, rom_root):
    """Keep only results with an emulator ROM folder on this SD card."""
    result = []
    for game in games:
        try:
            destination(rom_root, game)
        except SourceError:
            continue
        result.append(game)
    return result


def ensure_pgm_bios(game, rom_path):
    """Put the device's existing PGM BIOS where its MAME core looks for it."""
    if game.system != "mame" or rom_path.suffix.lower() != ".zip":
        return
    try:
        with zipfile.ZipFile(rom_path) as archive:
            names = (item.filename.lower() for item in archive.infolist())
            if not any(name.startswith(("pgm_a", "pgm_b", "pgm_t")) for name in names):
                return
    except zipfile.BadZipFile:
        return
    bios = rom_path.parent.parent.parent / "RetroArch/.retroarch/system/pgm.zip"
    destination = rom_path.parent / "pgm.zip"
    if bios.is_file() and not destination.exists():
        shutil.copyfile(bios, destination)


def archive_members(archive):
    archiver = Path(__file__).parent / "bin/7zzs"
    if not archiver.is_file():
        raise SourceError("Archive extractor is missing from ROM Search")
    try:
        result = subprocess.run([str(archiver), "l", "-slt", str(archive)],
                                capture_output=True, timeout=60)
    except subprocess.TimeoutExpired as exc:
        raise SourceError("Archive listing timed out") from exc
    if result.returncode or len(result.stdout) > 4_000_000:
        raise SourceError("Cannot read the downloaded archive")
    listing = result.stdout.decode("utf-8", "replace").replace("\r\n", "\n")
    if "\n----------\n" not in listing:
        raise SourceError("Downloaded file is not a supported archive")
    members = []
    total = 0
    for block in listing.split("\n----------\n", 1)[1].strip().split("\n\n"):
        fields = dict(line.split(" = ", 1) for line in block.splitlines() if " = " in line)
        name = fields.get("Path", "")
        if not name:
            continue
        parts = name.replace("\\", "/").split("/")
        if (name.startswith(("/", "\\", "-")) or "\ufffd" in name
                or any(part in ("", ".", "..") for part in parts)
                or ":" in parts[0] or any(char in name for char in '*?"<>|\x00')):
            raise SourceError("Archive contains an unsafe filename")
        attributes = fields.get("Attributes", "")
        if fields.get("Folder") == "+" or attributes.startswith("D "):
            continue
        if fields.get("Encrypted") == "+" or " l" in attributes:
            raise SourceError("Encrypted or linked archive members are unsupported")
        try:
            size = int(fields["Size"])
        except (KeyError, ValueError) as exc:
            raise SourceError("Archive has an invalid file size") from exc
        total += size
        if size < 0 or total > MAX_ROM or len(members) >= 256:
            raise SourceError("Archive expands beyond the supported limit")
        basename = parts[-1]
        if basename.endswith((" ", ".")):
            raise SourceError("Archive filename is unsupported on this SD card")
        members.append({"path": name, "name": basename,
                        "parent": tuple(parts[:-1]),
                        "suffix": PurePosixPath(basename).suffix.lower(), "size": size})
    return archiver, members


def select_archive_members(members, folder):
    playable = [item for item in members if item["suffix"] in PLAYABLE_EXT[folder]]
    if not playable:
        raise SourceError("Archive contains no playable %s game file" % folder)
    if folder in DISC_FOLDERS:
        playlists = [item for item in playable if item["suffix"] == ".m3u"]
        descriptors = playlists or [item for item in playable if item["suffix"] in DISC_DESCRIPTORS]
        if descriptors:
            if len(descriptors) != 1:
                raise SourceError("Archive has multiple disc launch files")
            primary = descriptors[0]
            companions = DISC_COMPANIONS | DISC_DESCRIPTORS
            selected = [item for item in members if item is primary or
                        (item["parent"] == primary["parent"] and item["suffix"] in companions)]
        else:
            if len(playable) != 1:
                raise SourceError("Archive has multiple game files")
            primary, selected = playable[0], playable
    else:
        if len(playable) != 1:
            raise SourceError("Archive has multiple game files")
        primary, selected = playable[0], playable
    if len({item["name"].lower() for item in selected}) != len(selected):
        raise SourceError("Archive has duplicate game filenames")
    return primary, selected


def install_archive(archive, target_dir, folder):
    archiver, members = archive_members(archive)
    primary, selected = select_archive_members(members, folder)
    targets = [target_dir / item["name"] for item in selected]
    if any(path.exists() for path in targets):
        raise SourceError("A game file already exists in %s" % folder)
    with tempfile.TemporaryDirectory(prefix=".rom-search-install-", dir=target_dir) as temporary:
        staged = []
        for item in selected:
            path = Path(temporary) / item["name"]
            try:
                with path.open("xb") as output:
                    result = subprocess.run([str(archiver), "x", "-so", "-bd",
                                             str(archive), item["path"]],
                                            stdout=output, stderr=subprocess.PIPE,
                                            timeout=1800)
            except subprocess.TimeoutExpired as exc:
                raise SourceError("Game extraction timed out") from exc
            if result.returncode or path.stat().st_size != item["size"]:
                raise SourceError("Game extraction failed or was incomplete")
            staged.append(path)
        installed = []
        try:
            for source, target in zip(staged, targets):
                os.replace(source, target)
                installed.append(target)
        except OSError:
            for path in installed:
                path.unlink(missing_ok=True)
            raise
    return target_dir / primary["name"]


def existing_playable_for_archive(target, folder):
    preferred = (".m3u", ".cue", ".gdi", ".mds", ".ccd",
                 ".iso", ".cso", ".pbp", ".chd")
    ordered = [ext for ext in preferred if ext in PLAYABLE_EXT[folder]]
    ordered += sorted(PLAYABLE_EXT[folder] - set(ordered))
    for extension in ordered:
        candidate = target.with_suffix(extension)
        if candidate.is_file() and candidate.stat().st_size:
            return candidate
    return None


def download(game, rom_root, progress=None, status=None):
    target_dir = destination(rom_root, game)
    if game.source == "CoolROM":
        url = coolrom_download_url(game)
    elif game.source == "RomsFun":
        url = romsfun_download_url(game)
    else:
        raise SourceError("Unknown source")
    parsed = urllib.parse.urlsplit(url)
    allowed_host = parsed.hostname
    expected_host = (allowed_host == "dl.coolrom.com" if game.source == "CoolROM"
                     else bool(re.fullmatch(r"sto[1-9][0-9]*\.romsforever\.co", allowed_host or "")))
    if parsed.scheme != "https" or not expected_host:
        raise SourceError("Unexpected download host")
    if game.source == "CoolROM":
        match = re.match(r"^/roms/[^/]+/([^/]+)/[^/]+/\d+/?$", parsed.path)
        if not match:
            raise SourceError("Unexpected download URL format")
        filename = Path(urllib.parse.unquote(match.group(1))).name
    else:
        filename = Path(urllib.parse.unquote(parsed.path.rstrip("/").split("/")[-1])).name
    suffix = Path(filename).suffix.lower()
    if suffix not in ALLOWED_EXT or filename in ("", ".", ".."):
        raise SourceError("Unsupported download filename")
    folder = target_dir.name
    if folder in ARCADE_FOLDERS and suffix != ".zip":
        raise SourceError("This arcade emulator requires a ZIP ROM set")
    if folder in DISC_FOLDERS and suffix in DISC_DESCRIPTORS:
        raise SourceError("Disc track files must come together in an archive")
    if suffix not in ARCHIVE_EXT and suffix not in PLAYABLE_EXT[folder]:
        raise SourceError("%s cannot open %s files" % (folder, suffix))
    target = target_dir / filename
    if suffix in ARCHIVE_EXT and folder not in ARCADE_FOLDERS:
        installed = existing_playable_for_archive(target, folder)
        if installed:
            return installed
    if target.exists():
        if suffix in ARCHIVE_EXT and folder not in ARCADE_FOLDERS:
            if status:
                status("Extracting game for %s..." % folder)
            installed = install_archive(target, target_dir, folder)
            target.unlink()
            return installed
        ensure_pgm_bios(game, target)
        return target
    partial = target.with_name(target.name + ".partial")
    if partial.exists():
        partial.unlink()
    total = 0
    try:
        with urllib.request.urlopen(request(url, referer=game.url), timeout=30) as response:
            if urllib.parse.urlsplit(response.url).hostname != allowed_host:
                raise SourceError("Download redirected to unexpected host")
            content_type = response.headers.get("Content-Type", "").lower()
            if "text/html" in content_type:
                raise SourceError("Site returned an HTML page instead of a ROM")
            length = int(response.headers.get("Content-Length", "0") or "0")
            if length > MAX_ROM:
                raise SourceError("Download exceeds size limit")
            with open(partial, "xb") as output:
                while True:
                    chunk = response.read(256 * 1024)
                    if not chunk:
                        break
                    total += len(chunk)
                    if total > MAX_ROM:
                        raise SourceError("Download exceeds size limit")
                    output.write(chunk)
                    if progress:
                        progress(total, length)
                output.flush()
                os.fsync(output.fileno())
        if not total or (length and total != length):
            raise SourceError("Download incomplete")
        if suffix in ARCHIVE_EXT and folder not in ARCADE_FOLDERS:
            if status:
                status("Extracting game for %s..." % folder)
            installed = install_archive(partial, target_dir, folder)
            partial.unlink()
            return installed
        os.replace(partial, target)
        ensure_pgm_bios(game, target)
        return target
    except Exception:
        partial.unlink(missing_ok=True)
        raise


def thumbnail_url(game):
    page = read_page(game.url)
    match = re.search(r'<meta\s+property="og:image"\s+content="([^"]+)"', page)
    if not match:
        raise SourceError("No thumbnail on game page")
    url = urllib.parse.urljoin(game.url, html.unescape(match.group(1)))
    parsed = urllib.parse.urlsplit(url)
    expected = "coolrom.com" if game.source == "CoolROM" else "romsfun.com"
    if parsed.scheme != "https" or parsed.hostname != expected:
        raise SourceError("Unexpected thumbnail host")
    if Path(parsed.path).suffix.lower() not in (".png", ".jpg", ".jpeg"):
        raise SourceError("Unsupported thumbnail format")
    return urllib.parse.urlunsplit((parsed.scheme, parsed.netloc,
                                   urllib.parse.quote(parsed.path, safe="/%"),
                                   parsed.query, parsed.fragment))


def save_thumbnail(game, rom_path, img_root):
    """Save a PNG with the ROM's base filename for stock MainUI."""
    folder = SYSTEM_FOLDERS.get(game.system)
    if not folder:
        raise SourceError("No emulator folder mapped for %s" % game.system)
    image_dir = Path(img_root) / folder
    if not image_dir.is_dir():
        raise SourceError("Image folder missing: %s" % image_dir)
    target = image_dir / (Path(rom_path).stem + ".png")
    if target.exists():
        return target
    gm = shutil.which("gm")
    if not gm:
        raise SourceError("GraphicsMagick is unavailable")
    url = thumbnail_url(game)
    with tempfile.TemporaryDirectory(prefix=".rom-art-", dir=image_dir) as temporary:
        source = Path(temporary) / "source"
        converted = Path(temporary) / "thumbnail.png"
        try:
            with urllib.request.urlopen(request(url, referer=game.url), timeout=20) as response:
                if urllib.parse.urlsplit(response.url).hostname != urllib.parse.urlsplit(url).hostname:
                    raise SourceError("Thumbnail redirected to unexpected host")
                if response.headers.get("Content-Type", "").split(";", 1)[0].lower() not in ("image/jpeg", "image/png"):
                    raise SourceError("Site returned a non-image thumbnail")
                total = 0
                with source.open("xb") as stream:
                    while True:
                        chunk = response.read(64 * 1024)
                        if not chunk:
                            break
                        total += len(chunk)
                        if total > MAX_ART:
                            raise SourceError("Thumbnail exceeds size limit")
                        stream.write(chunk)
                if not total:
                    raise SourceError("Thumbnail is empty")
            size = subprocess.run([gm, "identify", "-format", "%w %h", str(source)],
                                  capture_output=True, text=True, timeout=15, check=True).stdout
            width, height = (int(part) for part in size.split())
            if width > 4096 or height > 4096 or width < 1 or height < 1:
                raise SourceError("Thumbnail dimensions are unsupported")
            subprocess.run([gm, "convert", str(source), "-auto-orient", "-resize",
                            "320x320>", "-strip", str(converted)],
                           capture_output=True, timeout=30, check=True)
            if not converted.read_bytes().startswith(b"\x89PNG\r\n\x1a\n"):
                raise SourceError("Thumbnail conversion failed")
            if not target.exists():
                os.replace(converted, target)
            return target
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as exc:
            raise SourceError("Thumbnail conversion failed: %s" % exc) from exc
