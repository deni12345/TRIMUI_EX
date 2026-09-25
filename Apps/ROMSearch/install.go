package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const maxROM int64 = 8 << 30

var playable = map[string]string{
	"PSP": ".iso .cso .pbp .chd", "PS": ".cue .bin .img .mdf .pbp .chd .m3u .toc .cbn",
	"SS": ".cue .iso .chd .bin .img .mds .ccd", "DC": ".gdi .cdi .chd .cue", "SEGACD": ".cue .iso .chd .bin .img",
	"FC": ".nes .fds .unf .unif", "SFC": ".sfc .smc .fig .gd3 .gd7 .dx2 .bsx .swc",
	"GB": ".gb .gbc", "GBC": ".gbc .gb", "GBA": ".gba .agb .gbz", "N64": ".n64 .z64 .v64", "NDS": ".nds",
	"MD": ".bin .gen .md .smd .32x", "MS": ".sms .rom .gg .sg", "GG": ".gg",
	"ATARI2600": ".a26 .bin .rom", "ATARI5200": ".a52 .bin .rom", "ATARI7800": ".a78 .bin .rom",
	"LYNX": ".lnx .lyx", "NGP": ".ngp .ngc", "C64": ".d64 .t64 .tap .crt .prg .p00",
	"MAME": ".zip", "NEOGEO": ".zip", "CPS1": ".zip", "CPS2": ".zip",
}
var archiveExt = map[string]bool{".zip": true, ".7z": true, ".rar": true}
var arcade = map[string]bool{"MAME": true, "NEOGEO": true, "CPS1": true, "CPS2": true}
var disc = map[string]bool{"PS": true, "SS": true, "DC": true, "SEGACD": true}
var descriptor = map[string]bool{".cue": true, ".gdi": true, ".m3u": true, ".mds": true, ".ccd": true}
var companion = map[string]bool{".bin": true, ".img": true, ".iso": true, ".raw": true, ".wav": true, ".ape": true, ".flac": true, ".mp3": true, ".chd": true, ".mdf": true, ".sub": true}

func hasExt(folder, ext string) bool { return strings.Contains(" "+playable[folder]+" ", " "+ext+" ") }
func downloadLink(g Game) (string, string, error) {
	page, e := readPage(g.URL)
	if e != nil {
		return "", "", e
	}
	switch g.Source {
	case "CoolROM":
		re := regexp.MustCompile(`https://dl\.coolrom\.com/roms/[^"'\s<>]+`)
		raw := strings.ReplaceAll(re.FindString(page), `\/`, "/")
		if raw == "" {
			return "", "", fmt.Errorf("CoolROM download link missing")
		}
		u, e := url.Parse(strings.ReplaceAll(raw, "&amp;", "&"))
		if e != nil {
			return "", "", e
		}
		m := regexp.MustCompile(`^/roms/[^/]+/([^/]+)/[^/]+/[0-9]+/?$`).FindStringSubmatch(u.Path)
		if u.Hostname() != "dl.coolrom.com" || len(m) != 2 {
			return "", "", fmt.Errorf("unexpected CoolROM download URL")
		}
		name, _ := url.PathUnescape(m[1])
		return u.String(), name, nil
	case "RomsFun":
		re := regexp.MustCompile(`https://romsfun\.com/download/[^"/]+`)
		next := re.FindString(page)
		if next == "" {
			return "", "", fmt.Errorf("RomsFun download page missing")
		}
		second, e := readPage(next + "/1?v=" + strconv.FormatInt(time.Now().UnixNano(), 10))
		if e != nil {
			return "", "", e
		}
		re = regexp.MustCompile(`<a\s+href="([^"]+)"\s+id="download-link"`)
		m := re.FindStringSubmatch(second)
		if len(m) != 2 {
			return "", "", fmt.Errorf("RomsFun file link missing")
		}
		raw := strings.ReplaceAll(m[1], "&amp;", "&")
		u, e := url.Parse(raw)
		if e != nil {
			return "", "", e
		}
		if !regexp.MustCompile(`^sto[1-9][0-9]*\.romsforever\.co$`).MatchString(u.Hostname()) {
			return "", "", fmt.Errorf("unexpected RomsFun host")
		}
		name, _ := url.PathUnescape(path.Base(u.Path))
		return raw, name, nil
	default:
		m := regexp.MustCompile(`data-media-id="([0-9]+)"`).FindStringSubmatch(page)
		if len(m) != 2 {
			return "", "", fmt.Errorf("RomsGames media ID missing")
		}
		req, e := newRequest("POST", g.URL+"?download", g.URL, strings.NewReader("mediaId="+m[1]))
		if e != nil {
			return "", "", e
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		resp, e := client.Do(req)
		if e != nil {
			return "", "", e
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return "", "", fmt.Errorf("RomsGames download HTTP %d", resp.StatusCode)
		}
		var payload struct{ DownloadURL, DownloadName string }
		if e = json.NewDecoder(io.LimitReader(resp.Body, 100_000)).Decode(&payload); e != nil {
			return "", "", e
		}
		u, e := url.Parse(payload.DownloadURL)
		if e != nil {
			return "", "", e
		}
		if u.Scheme != "https" || u.Hostname() != "static.romsgames.net" {
			return "", "", fmt.Errorf("unexpected RomsGames host")
		}
		name, _ := url.QueryUnescape(payload.DownloadName)
		if name == "" {
			return "", "", fmt.Errorf("RomsGames filename missing")
		}
		q := u.Query()
		q.Set("mediaId", m[1])
		q.Set("attach", payload.DownloadName)
		u.RawQuery = q.Encode()
		return u.String(), name, nil
	}
}
func safeName(name string) bool {
	return name != "" && name != "." && name != ".." && name == filepath.Base(name) && !strings.HasPrefix(name, "-") && !strings.HasSuffix(name, ".") && !strings.HasSuffix(name, " ") && !strings.ContainsAny(name, "/\\:*?\"<>|\x00")
}
func cleanDownloadName(name string) string {
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, "/\\") {
		return ""
	}
	name = strings.NewReplacer(":", " -", "*", "-", "?", "", "\"", "", "<", "", ">", "", "|", "-", "\x00", "").Replace(name)
	return strings.Trim(name, " .")
}
func freeBytes(p string) int64 {
	var st syscall.Statfs_t
	if syscall.Statfs(p, &st) != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize)
}
func stageRoot(size int64) string {
	p := "/mnt/UDISK"
	if size > 0 && size < 2<<30 && freeBytes(p) > size*3+(200<<20) {
		return p
	}
	return ""
}
func remoteLength(raw, referer string) int64 {
	resp, e := openDownload(raw, referer, "bytes=0-0")
	if e != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode == 206 {
		m := regexp.MustCompile(`^bytes 0-0/([0-9]+)$`).FindStringSubmatch(resp.Header.Get("Content-Range"))
		if len(m) == 2 {
			size, _ := strconv.ParseInt(m[1], 10, 64)
			return size
		}
	}
	if resp.StatusCode == 200 {
		return resp.ContentLength
	}
	return 0
}
func openDownload(raw, referer, rangeHeader string) (*http.Response, error) {
	req, e := newRequest("GET", raw, referer, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept-Encoding", "identity")
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	resp, e := client.Do(req)
	if e != nil {
		return nil, e
	}
	expected, _ := url.Parse(raw)
	if resp.Request.URL.Hostname() != expected.Hostname() {
		resp.Body.Close()
		return nil, fmt.Errorf("download redirected to unexpected host")
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		resp.Body.Close()
		return nil, fmt.Errorf("site returned HTML instead of a ROM")
	}
	return resp, nil
}
func copyStream(dst *os.File, src io.Reader, max int64, progress func(int64, int64), total int64) error {
	buf := make([]byte, 1024*1024)
	var done int64
	for {
		n, e := src.Read(buf)
		if n > 0 {
			done += int64(n)
			if done > max {
				return fmt.Errorf("download exceeds size limit")
			}
			if _, w := dst.Write(buf[:n]); w != nil {
				return w
			}
			if progress != nil {
				progress(done, total)
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
	}
	if done == 0 || (total > 0 && done != total) {
		return fmt.Errorf("download incomplete")
	}
	return nil
}
func fetchFile(raw, referer, target string, progress func(int64, int64)) error {
	resp, e := openDownload(raw, referer, "bytes=0-0")
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	total := resp.ContentLength
	rangeOK := false
	if resp.StatusCode == 206 {
		m := regexp.MustCompile(`^bytes 0-0/([0-9]+)$`).FindStringSubmatch(resp.Header.Get("Content-Range"))
		if len(m) == 2 {
			total, _ = strconv.ParseInt(m[1], 10, 64)
			rangeOK = total > 4<<20 && total <= maxROM
		}
	}
	if total > maxROM {
		return fmt.Errorf("download exceeds size limit")
	}
	out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer out.Close()
	if rangeOK {
		resp.Body.Close()
		if e = out.Truncate(total); e != nil {
			return e
		}
		const workers = 4
		var wg sync.WaitGroup
		errs := make(chan error, workers)
		var mu sync.Mutex
		var done int64
		for i := 0; i < workers; i++ {
			start := int64(i) * total / workers
			end := int64(i+1)*total/workers - 1
			wg.Add(1)
			go func() {
				defer wg.Done()
				r, er := openDownload(raw, referer, fmt.Sprintf("bytes=%d-%d", start, end))
				if er != nil {
					errs <- er
					return
				}
				defer r.Body.Close()
				if r.StatusCode != 206 || !strings.HasPrefix(r.Header.Get("Content-Range"), fmt.Sprintf("bytes %d-%d/", start, end)) {
					errs <- fmt.Errorf("server did not honor byte range")
					return
				}
				buf := make([]byte, 1024*1024)
				pos := start
				for {
					n, er := r.Body.Read(buf)
					if n > 0 {
						if pos+int64(n) > end+1 {
							errs <- fmt.Errorf("range exceeded")
							return
						}
						if _, w := out.WriteAt(buf[:n], pos); w != nil {
							errs <- w
							return
						}
						pos += int64(n)
						mu.Lock()
						done += int64(n)
						if progress != nil {
							progress(done, total)
						}
						mu.Unlock()
					}
					if er == io.EOF {
						break
					}
					if er != nil {
						errs <- er
						return
					}
				}
				if pos != end+1 {
					errs <- fmt.Errorf("range incomplete")
				}
			}()
		}
		wg.Wait()
		close(errs)
		if len(errs) == 0 {
			return nil
		}
		if e = out.Truncate(0); e != nil {
			return e
		}
		if _, e = out.Seek(0, 0); e != nil {
			return e
		}
		resp, e = openDownload(raw, referer, "")
		if e != nil {
			return e
		}
		defer resp.Body.Close()
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	return copyStream(out, resp.Body, maxROM, progress, resp.ContentLength)
}

type member struct {
	Name, Base, Ext, Parent string
	Size                    int64
}

func archiveMembers(archive string) ([]member, error) {
	out, e := exec.Command(filepath.Join(appDir(), "bin", "7zzs"), "l", "-slt", archive).Output()
	if e != nil {
		return nil, fmt.Errorf("cannot list archive: %w", e)
	}
	parts := strings.SplitN(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n----------\n", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid archive")
	}
	var members []member
	var total int64
	for _, block := range strings.Split(strings.TrimSpace(parts[1]), "\n\n") {
		fields := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			p := strings.SplitN(line, " = ", 2)
			if len(p) == 2 {
				fields[p[0]] = p[1]
			}
		}
		name := strings.ReplaceAll(fields["Path"], "\\", "/")
		if name == "" {
			continue
		}
		if fields["Folder"] == "+" {
			continue
		}
		bits := strings.Split(name, "/")
		for _, part := range bits {
			if !safeName(part) {
				return nil, fmt.Errorf("unsafe archive filename")
			}
		}
		if fields["Encrypted"] == "+" || strings.Contains(fields["Attributes"], " l") {
			return nil, fmt.Errorf("encrypted or linked archive unsupported")
		}
		size, e := strconv.ParseInt(fields["Size"], 10, 64)
		if e != nil || size < 0 {
			return nil, fmt.Errorf("archive size invalid")
		}
		total += size
		if total > maxROM || len(members) >= 256 {
			return nil, fmt.Errorf("archive expands beyond limit")
		}
		base := bits[len(bits)-1]
		members = append(members, member{name, base, strings.ToLower(filepath.Ext(base)), strings.Join(bits[:len(bits)-1], "/"), size})
	}
	return members, nil
}
func selectedMembers(members []member, folder string) (member, []member, error) {
	var playableMembers []member
	for _, m := range members {
		if hasExt(folder, m.Ext) {
			playableMembers = append(playableMembers, m)
		}
	}
	if len(playableMembers) == 0 {
		return member{}, nil, fmt.Errorf("archive has no playable %s file", folder)
	}
	if !disc[folder] {
		if len(playableMembers) != 1 {
			return member{}, nil, fmt.Errorf("archive has multiple game files")
		}
		return playableMembers[0], playableMembers, nil
	}
	var playlists, desc []member
	for _, m := range playableMembers {
		if m.Ext == ".m3u" {
			playlists = append(playlists, m)
		} else if descriptor[m.Ext] {
			desc = append(desc, m)
		}
	}
	if len(playlists) > 0 {
		desc = playlists
	}
	if len(desc) > 1 {
		return member{}, nil, fmt.Errorf("archive has multiple disc launch files")
	}
	if len(desc) == 1 {
		var selected []member
		for _, m := range members {
			if m.Parent == desc[0].Parent && (m.Name == desc[0].Name || companion[m.Ext] || descriptor[m.Ext]) {
				selected = append(selected, m)
			}
		}
		return desc[0], selected, nil
	}
	if len(playableMembers) != 1 {
		return member{}, nil, fmt.Errorf("archive has multiple game files")
	}
	return playableMembers[0], playableMembers, nil
}
func appDir() string { exe, _ := os.Executable(); return filepath.Dir(exe) }
func copyFile(source, target string) error {
	in, e := os.Open(source)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		return e
	}
	_, e = io.CopyBuffer(out, in, make([]byte, 1024*1024))
	closeErr := out.Close()
	if e != nil {
		return e
	}
	return closeErr
}
func installArchive(archive, targetDir, folder string) (string, error) {
	members, e := archiveMembers(archive)
	if e != nil {
		return "", e
	}
	primary, selected, e := selectedMembers(members, folder)
	if e != nil {
		return "", e
	}
	seen := map[string]bool{}
	for _, m := range selected {
		key := strings.ToLower(m.Base)
		if seen[key] {
			return "", fmt.Errorf("duplicate archive filenames")
		}
		seen[key] = true
		if _, e = os.Stat(filepath.Join(targetDir, m.Base)); e == nil {
			return "", fmt.Errorf("%s already exists", m.Base)
		}
	}
	var expanded int64
	for _, m := range selected {
		expanded += m.Size
	}
	extractBase := filepath.Dir(archive)
	if freeBytes(extractBase) < expanded+(100<<20) {
		extractBase = targetDir
	}
	stage, e := os.MkdirTemp(extractBase, "romsearch-extract-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(stage)
	args := []string{"x", "-bd", "-y", "-o" + stage, archive}
	for _, m := range selected {
		args = append(args, m.Name)
	}
	cmd := exec.Command(filepath.Join(appDir(), "bin", "7zzs"), args...)
	if out, e := cmd.CombinedOutput(); e != nil {
		return "", fmt.Errorf("extract failed: %s", string(out))
	}
	pending, e := os.MkdirTemp(targetDir, ".romsearch-install-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(pending)
	for _, m := range selected {
		source := filepath.Join(stage, filepath.FromSlash(m.Name))
		info, e := os.Lstat(source)
		if e != nil || !info.Mode().IsRegular() || info.Size() != m.Size {
			return "", fmt.Errorf("extracted file invalid: %s", m.Base)
		}
		if extractBase == targetDir {
			e = os.Rename(source, filepath.Join(pending, m.Base))
		} else {
			e = copyFile(source, filepath.Join(pending, m.Base))
		}
		if e != nil {
			return "", e
		}
	}
	var installed []string
	for _, m := range selected {
		target := filepath.Join(targetDir, m.Base)
		if e = os.Rename(filepath.Join(pending, m.Base), target); e != nil {
			for _, p := range installed {
				os.Remove(p)
			}
			return "", e
		}
		installed = append(installed, target)
	}
	return filepath.Join(targetDir, primary.Base), nil
}
func ensureBIOS(g Game, target string) {
	if g.System != "MAME" || filepath.Ext(target) != ".zip" {
		return
	}
	reader, e := zip.OpenReader(target)
	if e != nil {
		return
	}
	defer reader.Close()
	found := false
	for _, f := range reader.File {
		if strings.HasPrefix(strings.ToLower(f.Name), "pgm_") {
			found = true
			break
		}
	}
	if !found {
		return
	}
	bios := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(target))), "RetroArch", ".retroarch", "system", "pgm.zip")
	destination := filepath.Join(filepath.Dir(target), "pgm.zip")
	if _, e = os.Stat(destination); e == nil {
		return
	}
	_ = copyFile(bios, destination)
}
func install(g Game, romRoot, imgRoot string, progress func(int64, int64), status func(string)) (string, error) {
	targetDir := filepath.Join(romRoot, g.System)
	if _, e := os.Stat(targetDir); e != nil {
		return "", fmt.Errorf("emulator ROM folder missing")
	}
	raw, name, e := downloadLink(g)
	if e != nil {
		return "", e
	}
	name = cleanDownloadName(name)
	if !safeName(name) {
		return "", fmt.Errorf("unsupported download filename")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if arcade[g.System] && ext != ".zip" {
		return "", fmt.Errorf("%s requires a ZIP ROM set", g.System)
	}
	if !archiveExt[ext] && !hasExt(g.System, ext) {
		return "", fmt.Errorf("%s cannot open %s files", g.System, ext)
	}
	if disc[g.System] && descriptor[ext] {
		return "", fmt.Errorf("disc tracks must come together in an archive")
	}
	target := filepath.Join(targetDir, name)
	if st, e := os.Stat(target); e == nil && st.Size() > 0 {
		if archiveExt[ext] && !arcade[g.System] {
			status("Extracting for " + g.System + "...")
			installed, extractErr := installArchive(target, targetDir, g.System)
			if extractErr != nil {
				return "", extractErr
			}
			os.Remove(target)
			target = installed
		}
		ensureBIOS(g, target)
		if thumbnailErr := saveThumbnail(g, target, imgRoot); thumbnailErr != nil {
			return target, fmt.Errorf("ROM saved; thumbnail: %w", thumbnailErr)
		}
		return target, nil
	}
	stageBase := stageRoot(remoteLength(raw, g.URL))
	if stageBase == "" {
		stageBase = targetDir
	}
	stage, e := os.MkdirTemp(stageBase, ".romsearch-download-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(stage)
	payload := filepath.Join(stage, name)
	if e = fetchFile(raw, g.URL, payload, progress); e != nil {
		return "", e
	}
	if archiveExt[ext] && !arcade[g.System] {
		status("Extracting for " + g.System + "...")
		target, e = installArchive(payload, targetDir, g.System)
		if e != nil {
			return "", e
		}
	} else {
		pending := filepath.Join(targetDir, "."+name+".partial")
		os.Remove(pending)
		if e = copyFile(payload, pending); e != nil {
			os.Remove(pending)
			return "", e
		}
		if e = os.Rename(pending, target); e != nil {
			os.Remove(pending)
			return "", e
		}
	}
	ensureBIOS(g, target)
	status("Saving thumbnail...")
	if e = saveThumbnail(g, target, imgRoot); e != nil {
		return target, fmt.Errorf("ROM saved; thumbnail: %w", e)
	}
	return target, nil
}
func saveThumbnail(g Game, rom, imgRoot string) error {
	target := filepath.Join(imgRoot, g.System, strings.TrimSuffix(filepath.Base(rom), filepath.Ext(rom))+".png")
	if _, e := os.Stat(target); e == nil {
		return nil
	}
	page, e := readPage(g.URL)
	if e != nil {
		return e
	}
	var raw string
	if g.Source == "RomsGames" {
		m := regexp.MustCompile(`<meta property="og:image" content="([^"]+)"`).FindStringSubmatch(page)
		if len(m) == 2 {
			raw = m[1]
		}
		if raw == "" {
			m = regexp.MustCompile(`<img src="(https://cache\.downloadroms\.io/static/[^"]+)"`).FindStringSubmatch(page)
			if len(m) == 2 {
				raw = m[1]
			}
		}
	} else {
		m := regexp.MustCompile(`<meta\s+property="og:image"\s+content="([^"]+)"`).FindStringSubmatch(page)
		if len(m) == 2 {
			raw = m[1]
		}
	}
	if raw == "" {
		return errors.New("no thumbnail")
	}
	u, e := url.Parse(strings.ReplaceAll(raw, "&amp;", "&"))
	if e != nil {
		return e
	}
	if u.Scheme != "https" || !(u.Hostname() == "coolrom.com" || u.Hostname() == "romsfun.com" || u.Hostname() == "cache.downloadroms.io") {
		return errors.New("unexpected thumbnail host")
	}
	req, e := newRequest("GET", u.String(), g.URL, nil)
	if e != nil {
		return e
	}
	resp, e := client.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("thumbnail HTTP %d", resp.StatusCode)
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20+1))
	if e != nil {
		return e
	}
	if len(data) > 4<<20 {
		return errors.New("thumbnail too large")
	}
	source, _, e := image.Decode(bytes.NewReader(data))
	if e != nil {
		return e
	}
	bounds := source.Bounds()
	if bounds.Dx() < 1 || bounds.Dy() < 1 || bounds.Dx() > 4096 || bounds.Dy() > 4096 {
		return errors.New("thumbnail dimensions invalid")
	}
	width, height := bounds.Dx(), bounds.Dy()
	if width > 320 || height > 320 {
		scale := min(float64(320)/float64(width), float64(320)/float64(height))
		width = max(1, int(float64(width)*scale))
		height = max(1, int(float64(height)*scale))
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sx := bounds.Min.X + x*bounds.Dx()/width
			sy := bounds.Min.Y + y*bounds.Dy()/height
			dst.Set(x, y, source.At(sx, sy))
		}
	}
	tmp := target + ".partial"
	os.Remove(tmp)
	file, e := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		return e
	}
	e = png.Encode(file, dst)
	closeErr := file.Close()
	if e != nil {
		os.Remove(tmp)
		return e
	}
	if closeErr != nil {
		os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, target)
}
