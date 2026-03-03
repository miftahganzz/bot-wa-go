//go:build ignore

package archive_music

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Archived snapshot of macOS music feature set (.music ...).
// This file is intentionally excluded from normal builds.

type musicTrackRef struct {
	PersistentID string
	Title        string
	Artist       string
}

type onlineTrackRef struct {
	Title      string
	Artist     string
	Album      string
	TrackURL   string
	PreviewURL string
}

type nowPlayingInfo struct {
	App    string
	State  string
	Title  string
	Artist string
	Album  string
	Pos    float64
	Dur    float64
}

var musicCacheMu sync.Mutex
var musicSearchCache []musicTrackRef
var musicQueue []musicTrackRef
var musicOnlineCache []onlineTrackRef

var musicPreviewMu sync.Mutex
var musicPreviewCmd *exec.Cmd

func handleMacMusic(args []string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", errors.New("fitur music hanya untuk macOS")
	}
	if _, err := exec.LookPath("osascript"); err != nil {
		return "", errors.New("osascript tidak ditemukan")
	}

	action := "status"
	if len(args) > 0 {
		action = strings.ToLower(strings.TrimSpace(args[0]))
	}

	switch action {
	case "status", "now", "nowplaying":
		info, err := getMacNowPlaying()
		if err != nil {
			return "", err
		}
		return formatNowPlayingText(info), nil
	case "list":
		limit := 15
		if len(args) > 1 {
			n, err := strconv.Atoi(strings.TrimSpace(args[1]))
			if err == nil && n > 0 {
				if n > 50 {
					n = 50
				}
				limit = n
			}
		}
		tracks, total, err := queryLibraryTracks("", limit)
		if err != nil {
			return "", err
		}
		if len(tracks) == 0 {
			return "Library kosong.", nil
		}
		musicCacheMu.Lock()
		musicSearchCache = append([]musicTrackRef(nil), tracks...)
		musicCacheMu.Unlock()
		var bld strings.Builder
		bld.WriteString(fmt.Sprintf("🎵 Music Library (%d)\n\n", total))
		for i, t := range tracks {
			bld.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, t.Title, t.Artist))
		}
		return strings.TrimSpace(bld.String()), nil
	case "search":
		if len(args) < 2 {
			return "", errors.New("format: music search <keyword>")
		}
		keyword := strings.TrimSpace(strings.Join(args[1:], " "))
		tracks, total, err := queryLibraryTracks(keyword, 20)
		if err != nil {
			return "", err
		}
		if len(tracks) == 0 {
			return "Tidak ada hasil.", nil
		}
		musicCacheMu.Lock()
		musicSearchCache = append([]musicTrackRef(nil), tracks...)
		musicCacheMu.Unlock()
		var bld strings.Builder
		bld.WriteString(fmt.Sprintf("🔎 Hasil (%d)\n\n", total))
		for i, t := range tracks {
			bld.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, t.Title, t.Artist))
		}
		return strings.TrimSpace(bld.String()), nil
	case "playq":
		if len(args) < 2 {
			return "", errors.New("format: music playq <nomor>")
		}
		idx, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil || idx <= 0 {
			return "", errors.New("nomor invalid")
		}
		musicCacheMu.Lock()
		cache := append([]musicTrackRef(nil), musicSearchCache...)
		musicCacheMu.Unlock()
		if idx > len(cache) || len(cache) == 0 {
			return "", errors.New("cache kosong/nomor di luar range")
		}
		return playByPersistentID(cache[idx-1].PersistentID)
	case "osearch":
		if len(args) < 2 {
			return "", errors.New("format: music osearch <keyword>")
		}
		keyword := strings.TrimSpace(strings.Join(args[1:], " "))
		results, err := searchOnlineTracks(keyword, 15)
		if err != nil {
			return "", err
		}
		if len(results) == 0 {
			return "Tidak ada hasil online.", nil
		}
		musicCacheMu.Lock()
		musicOnlineCache = append([]onlineTrackRef(nil), results...)
		musicCacheMu.Unlock()
		var bld strings.Builder
		bld.WriteString("🌐 Online Search\n\n")
		for i, t := range results {
			bld.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, t.Title, t.Artist))
		}
		return strings.TrimSpace(bld.String()), nil
	case "oplay":
		if len(args) < 2 {
			return "", errors.New("format: music oplay <nomor>")
		}
		idx, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil || idx <= 0 {
			return "", errors.New("nomor invalid")
		}
		musicCacheMu.Lock()
		cache := append([]onlineTrackRef(nil), musicOnlineCache...)
		musicCacheMu.Unlock()
		if idx > len(cache) || len(cache) == 0 {
			return "", errors.New("cache online kosong/nomor di luar range")
		}
		target := cache[idx-1]
		if strings.TrimSpace(target.PreviewURL) != "" {
			if err := playPreviewNow(target.PreviewURL); err != nil {
				return "", err
			}
			return fmt.Sprintf("▶️ Playing preview: %s - %s", target.Title, target.Artist), nil
		}
		return "Preview URL tidak tersedia.", nil
	default:
		return "", errors.New("unknown action")
	}
}

func getMacNowPlaying() (*nowPlayingInfo, error) {
	raw, err := runAppleScript(
		`set appName to ""`,
		`set titleText to ""`,
		`set artistText to ""`,
		`set albumText to ""`,
		`set stateText to ""`,
		`set posText to "0"`,
		`set durText to "0"`,
		`tell application "System Events"`,
		`  set hasMusic to (name of processes) contains "Music"`,
		`  set hasSpotify to (name of processes) contains "Spotify"`,
		`end tell`,
		`if hasMusic then`,
		`  set appName to "Music"`,
		`  tell application "Music"`,
		`    set stateText to (player state as text)`,
		`    if stateText is not "stopped" then`,
		`      set titleText to (name of current track)`,
		`      set artistText to (artist of current track)`,
		`      set albumText to (album of current track)`,
		`      set posText to (player position as text)`,
		`      set durText to (duration of current track as text)`,
		`    end if`,
		`  end tell`,
		`else if hasSpotify then`,
		`  set appName to "Spotify"`,
		`  tell application "Spotify"`,
		`    set stateText to (player state as text)`,
		`    if stateText is not "stopped" then`,
		`      set titleText to (name of current track)`,
		`      set artistText to (artist of current track)`,
		`      set albumText to (album of current track)`,
		`      set posText to (player position as text)`,
		`      set durText to (duration of current track as text)`,
		`    end if`,
		`  end tell`,
		`else`,
		`  return "NO_PLAYER"`,
		`end if`,
		`return appName & "|" & stateText & "|" & titleText & "|" & artistText & "|" & albumText & "|" & posText & "|" & durText`,
	)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw) == "NO_PLAYER" {
		return &nowPlayingInfo{App: "-", State: "stopped"}, nil
	}
	parts := strings.Split(raw, "|")
	if len(parts) < 7 {
		return &nowPlayingInfo{App: "-", State: "stopped"}, nil
	}
	return &nowPlayingInfo{
		App:    strings.TrimSpace(parts[0]),
		State:  strings.TrimSpace(parts[1]),
		Title:  strings.TrimSpace(parts[2]),
		Artist: strings.TrimSpace(parts[3]),
		Album:  strings.TrimSpace(parts[4]),
		Pos:    parseFloatLoose(parts[5]),
		Dur:    parseFloatLoose(parts[6]),
	}, nil
}

func formatNowPlayingText(info *nowPlayingInfo) string {
	if info == nil || info.App == "-" {
		return "Tidak ada player aktif (Music/Spotify)."
	}
	if info.Title == "" {
		return fmt.Sprintf("%s [%s]", info.App, info.State)
	}
	return fmt.Sprintf("🎵 %s [%s]\n%s - %s\n%s %s/%s", info.App, info.State, info.Title, info.Artist, progressBar(info.Pos, info.Dur, 12), formatSeconds(info.Pos), formatSeconds(info.Dur))
}

func queryLibraryTracks(keyword string, limit int) ([]musicTrackRef, int, error) {
	keyword = strings.TrimSpace(keyword)
	raw, err := runAppleScript(
		`tell application "Music"`,
		`  set trks to {}`,
		`  try`,
		`    set trks to tracks of playlist "Songs"`,
		`  on error`,
		`    set trks to tracks of library playlist 1`,
		`  end try`,
		fmt.Sprintf(`  set lim to %d`, limit),
		fmt.Sprintf(`  set kw to %s`, appleString(strings.ToLower(keyword))),
		`  set outLines to {}`,
		`  set totalCount to 0`,
		`  repeat with t in trks`,
		`    set nm to (name of t) as text`,
		`    set ar to (artist of t) as text`,
		`    if kw is "" or (nm contains kw) or (ar contains kw) then`,
		`      set totalCount to totalCount + 1`,
		`      if (count of outLines) < lim then`,
		`        set end of outLines to ((persistent ID of t) as text) & tab & nm & tab & ar`,
		`      end if`,
		`    end if`,
		`  end repeat`,
		`  if (count of outLines) = 0 then return "TOTAL:" & totalCount`,
		`  set AppleScript's text item delimiters to linefeed`,
		`  set outText to outLines as text`,
		`  set AppleScript's text item delimiters to ""`,
		`  return "TOTAL:" & totalCount & linefeed & outText`,
		`end tell`,
	)
	if err != nil {
		return nil, 0, err
	}
	return parseTrackRows(raw), parseTotal(raw), nil
}

func parseTotal(raw string) int {
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "TOTAL:") {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimPrefix(lines[0], "TOTAL:"))
	return n
}

func parseTrackRows(raw string) []musicTrackRef {
	lines := strings.Split(raw, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "TOTAL:") {
		lines = lines[1:]
	}
	out := make([]musicTrackRef, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		parts := strings.Split(ln, "\t")
		if len(parts) < 3 {
			continue
		}
		out = append(out, musicTrackRef{
			PersistentID: strings.TrimSpace(parts[0]),
			Title:        strings.TrimSpace(parts[1]),
			Artist:       strings.TrimSpace(parts[2]),
		})
	}
	return out
}

func playByPersistentID(pid string) (string, error) {
	return runAppleScript(
		`tell application "Music"`,
		`  set trks to {}`,
		fmt.Sprintf(`  set trks to (every track of library playlist 1 whose (persistent ID is %s))`, appleString(pid)),
		`  if (count of trks) = 0 then return "Track tidak ditemukan di library."`,
		`  set t to item 1 of trks`,
		`  play t`,
		`  return "▶️ Playing: " & (name of t) & " - " & (artist of t)`,
		`end tell`,
	)
}

func searchOnlineTracks(keyword string, limit int) ([]onlineTrackRef, error) {
	u, _ := neturl.Parse("https://itunes.apple.com/search")
	q := u.Query()
	q.Set("term", keyword)
	q.Set("entity", "song")
	q.Set("limit", strconv.Itoa(limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "meow-bot/1.0")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("itunes status %d", resp.StatusCode)
	}
	var data struct {
		Results []struct {
			TrackName    string `json:"trackName"`
			ArtistName   string `json:"artistName"`
			Collection   string `json:"collectionName"`
			TrackViewURL string `json:"trackViewUrl"`
			PreviewURL   string `json:"previewUrl"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	out := make([]onlineTrackRef, 0, len(data.Results))
	for _, r := range data.Results {
		out = append(out, onlineTrackRef{
			Title:      strings.TrimSpace(r.TrackName),
			Artist:     strings.TrimSpace(r.ArtistName),
			Album:      strings.TrimSpace(r.Collection),
			TrackURL:   strings.TrimSpace(r.TrackViewURL),
			PreviewURL: strings.TrimSpace(r.PreviewURL),
		})
	}
	return out, nil
}

func playPreviewNow(previewURL string) error {
	if _, err := exec.LookPath("afplay"); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, previewURL, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	tmp, err := os.CreateTemp("", "meow-preview-*")
	if err != nil {
		return err
	}
	defer tmp.Close()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return err
	}
	musicPreviewMu.Lock()
	if musicPreviewCmd != nil && musicPreviewCmd.Process != nil {
		_ = musicPreviewCmd.Process.Kill()
	}
	cmd := exec.Command("afplay", tmp.Name())
	if err := cmd.Start(); err != nil {
		musicPreviewMu.Unlock()
		return err
	}
	musicPreviewCmd = cmd
	musicPreviewMu.Unlock()
	go func(path string, c *exec.Cmd) {
		_ = c.Wait()
		_ = os.Remove(path)
	}(tmp.Name(), cmd)
	return nil
}

func runAppleScript(lines ...string) (string, error) {
	args := make([]string, 0, len(lines)*2)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		args = append(args, "-e", line)
	}
	cmd := exec.Command("osascript", args...)
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s == "" {
			return "", err
		}
		return "", errors.New(s)
	}
	return s, nil
}

func appleString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return `"` + s + `"`
}

func parseFloatLoose(s string) float64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	f, _ := strconv.ParseFloat(s, 64)
	if f < 0 {
		return 0
	}
	return f
}

func progressBar(pos, dur float64, width int) string {
	if dur <= 0 || width <= 0 {
		return "[------------]"
	}
	ratio := pos / dur
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("=", filled) + strings.Repeat("-", width-filled) + "]"
}

func formatSeconds(sec float64) string {
	n := int(sec + 0.5)
	if n < 0 {
		n = 0
	}
	m := n / 60
	s := n % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

