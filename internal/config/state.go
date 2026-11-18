package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Track represents a playable YouTube song/video item in the playlist.
type Track struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Artist       string `json:"artist"`
	DurationSec  int    `json:"duration_sec"`
	DurationText string `json:"duration_text"`
	Thumbnail    string `json:"thumbnail"`
}

// SavedPlaylist represents a user-saved named playlist.
type SavedPlaylist struct {
	Name      string  `json:"name"`
	Query     string  `json:"query,omitempty"`
	Tracks    []Track `json:"tracks"`
	CreatedAt int64   `json:"created_at"`
}

// AppState stores the persistent application state saved on exit and restored on startup.
type AppState struct {
	SearchQuery       string          `json:"search_query"`
	ContinuationToken string          `json:"continuation_token"`
	Playlist          []Track         `json:"playlist"`
	Overflow          []Track         `json:"overflow,omitempty"`
	CurrentIndex      int             `json:"current_index"`
	CurrentTrackID    string          `json:"current_track_id"`
	PositionSec       float64         `json:"position_sec"`
	WasPlaying        bool            `json:"was_playing"`
	VideoEnabled      bool            `json:"video_enabled"`
	PlaylistExpanded  bool            `json:"playlist_expanded"`
	AlwaysOnTop       bool            `json:"always_on_top"`
	Theme             string          `json:"theme"`
	Shuffle           bool            `json:"shuffle"`
	RepeatMode        string          `json:"repeat_mode"`
	LikedTracks       []Track         `json:"liked_tracks,omitempty"`
	SavedPlaylists    []SavedPlaylist `json:"saved_playlists,omitempty"`
	Volume            float64         `json:"volume"`
	Muted             bool            `json:"muted"`
	WindowX           int             `json:"window_x"`
	WindowY           int             `json:"window_y"`
}

// CookieEntry represents a browser cookie captured during Google/YouTube login.
type CookieEntry struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"http_only"`
}

// AuthState stores YouTube/Google account login credentials and cookies.
type AuthState struct {
	LoggedIn      bool              `json:"logged_in"`
	AccountName   string            `json:"account_name"`
	Email         string            `json:"email"`
	Cookies       map[string]string `json:"cookies"`
	CookieEntries []CookieEntry     `json:"cookie_entries,omitempty"`
	UpdatedAt     int64             `json:"updated_at"`
}

// DefaultAppState returns a clean initial state (audio-only by default).
func DefaultAppState() AppState {
	return AppState{
		Playlist:         []Track{},
		Overflow:         []Track{},
		CurrentIndex:     -1,
		VideoEnabled:     false,
		PlaylistExpanded: true,
		Theme:            "dark",
		RepeatMode:       "off",
		LikedTracks:      []Track{},
		SavedPlaylists:   []SavedPlaylist{},
		Volume:           0.8,
		WindowX:          -1,
		WindowY:          -1,
	}
}

// AppDataDir returns %APPDATA%\ytmusic (or os.UserConfigDir()\ytmusic).
func AppDataDir() (string, error) {
	base := os.Getenv("APPDATA")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve config dir: %w", err)
		}
	}
	return filepath.Join(base, "ytmusic"), nil
}

// StateFilePath returns the path to state.json.
func StateFilePath() (string, error) {
	dir, err := AppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

// AuthFilePath returns the path to auth.json.
func AuthFilePath() (string, error) {
	dir, err := AppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "auth.json"), nil
}

// CookiesTxtPath returns the path to the Netscape cookies.txt file for yt-dlp.
func CookiesTxtPath() (string, error) {
	dir, err := AppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cookies.txt"), nil
}

// WebViewProfileDir returns the persistent WebView2 user data folder for the main UI.
func WebViewProfileDir() (string, error) {
	dir, err := AppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "webview_profile"), nil
}

// AuthWebViewProfileDir returns the persistent WebView2 user data folder for the login helper.
func AuthWebViewProfileDir() (string, error) {
	dir, err := AppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "auth_webview"), nil
}

// LoadAppState loads state.json from %APPDATA%\ytmusic\state.json, returning defaults if missing.
func LoadAppState() (AppState, error) {
	path, err := StateFilePath()
	if err != nil {
		return DefaultAppState(), err
	}
	return LoadAppStateFrom(path)
}

// LoadAppStateFrom loads AppState from a given JSON file path.
func LoadAppStateFrom(path string) (AppState, error) {
	state := DefaultAppState()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return state, fmt.Errorf("read state file: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return DefaultAppState(), fmt.Errorf("parse state file: %w", err)
	}
	normalizeAppState(&state)
	return state, nil
}

func normalizeAppState(s *AppState) {
	if s.Playlist == nil {
		s.Playlist = []Track{}
	}
	if s.Overflow == nil {
		s.Overflow = []Track{}
	}
	if s.LikedTracks == nil {
		s.LikedTracks = []Track{}
	}
	if s.SavedPlaylists == nil {
		s.SavedPlaylists = []SavedPlaylist{}
	}
	switch s.RepeatMode {
	case "off", "all", "one":
	default:
		s.RepeatMode = "off"
	}
	switch s.Theme {
	case "dark", "light":
	default:
		s.Theme = "dark"
	}
	if s.Volume < 0 || s.Volume > 1 {
		s.Volume = 0.8
	}
	if len(s.Playlist) == 0 {
		s.CurrentIndex = -1
		s.CurrentTrackID = ""
		s.PositionSec = 0
		s.WasPlaying = false
		return
	}
	if s.CurrentIndex < -1 || s.CurrentIndex >= len(s.Playlist) {
		s.CurrentIndex = -1
	}
	// Re-sync index by track ID if possible.
	if s.CurrentTrackID != "" && (s.CurrentIndex < 0 || s.Playlist[s.CurrentIndex].ID != s.CurrentTrackID) {
		for i, t := range s.Playlist {
			if t.ID == s.CurrentTrackID {
				s.CurrentIndex = i
				break
			}
		}
	}
	if s.CurrentIndex >= 0 && s.CurrentIndex < len(s.Playlist) {
		s.CurrentTrackID = s.Playlist[s.CurrentIndex].ID
	}
	if s.PositionSec < 0 {
		s.PositionSec = 0
	}
}

// SaveAppState saves state.json atomically to %APPDATA%\ytmusic\state.json.
func SaveAppState(state AppState) error {
	path, err := StateFilePath()
	if err != nil {
		return err
	}
	return SaveAppStateTo(path, state)
}

// SaveAppStateTo saves AppState atomically to the given file path.
func SaveAppStateTo(path string, state AppState) error {
	normalizeAppState(&state)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	return writeFileAtomic(path, data)
}

// LoadAuthState loads auth.json from %APPDATA%\ytmusic\auth.json.
func LoadAuthState() (AuthState, error) {
	path, err := AuthFilePath()
	if err != nil {
		return AuthState{Cookies: map[string]string{}}, err
	}
	return LoadAuthStateFrom(path)
}

// LoadAuthStateFrom loads AuthState from a given file path.
func LoadAuthStateFrom(path string) (AuthState, error) {
	auth := AuthState{Cookies: map[string]string{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return auth, nil
		}
		return auth, fmt.Errorf("read auth file: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return auth, nil
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return AuthState{Cookies: map[string]string{}}, fmt.Errorf("parse auth file: %w", err)
	}
	if auth.Cookies == nil {
		auth.Cookies = map[string]string{}
	}
	return auth, nil
}

// SaveAuthState saves auth.json atomically and updates cookies.txt.
func SaveAuthState(auth AuthState) error {
	path, err := AuthFilePath()
	if err != nil {
		return err
	}
	if err := SaveAuthStateTo(path, auth); err != nil {
		return err
	}
	cookiesPath, err := CookiesTxtPath()
	if err == nil {
		_ = WriteNetscapeCookies(cookiesPath, auth)
	}
	return nil
}

// SaveAuthStateTo saves AuthState atomically to the given path.
func SaveAuthStateTo(path string, auth AuthState) error {
	if auth.Cookies == nil {
		auth.Cookies = map[string]string{}
	}
	data, err := json.MarshalIndent(auth, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal auth: %w", err)
	}
	return writeFileAtomic(path, data)
}

// ClearAuthState deletes auth.json and cookies.txt, and clears auth_webview if requested.
func ClearAuthState(clearWebViewProfile bool) error {
	if p, err := AuthFilePath(); err == nil {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if p, err := CookiesTxtPath(); err == nil {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if clearWebViewProfile {
		if dir, err := AuthWebViewProfileDir(); err == nil {
			_ = os.RemoveAll(dir)
		}
	}
	return nil
}

// WriteNetscapeCookies writes AuthState cookies in Netscape cookies.txt format for yt-dlp.
func WriteNetscapeCookies(path string, auth AuthState) error {
	if !auth.LoggedIn || (len(auth.CookieEntries) == 0 && len(auth.Cookies) == 0) {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n")
	b.WriteString("# Generated by ytmusic\n\n")

	if len(auth.CookieEntries) > 0 {
		for _, c := range auth.CookieEntries {
			domain := c.Domain
			if domain == "" {
				domain = ".youtube.com"
			}
			includeSubdomains := "FALSE"
			if strings.HasPrefix(domain, ".") {
				includeSubdomains = "TRUE"
			}
			cookiePath := c.Path
			if cookiePath == "" {
				cookiePath = "/"
			}
			secure := "FALSE"
			if c.Secure {
				secure = "TRUE"
			}
			exp := c.Expires
			if exp <= 0 {
				exp = 2147483647
			}
			name := c.Name
			if c.HTTPOnly {
				// Netscape format supports standard entries without #HttpOnly_ prefix for yt-dlp compatibility.
			}
			b.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
				domain, includeSubdomains, cookiePath, secure, exp, name, c.Value))
		}
	} else {
		for k, v := range auth.Cookies {
			b.WriteString(fmt.Sprintf(".youtube.com\tTRUE\t/\tTRUE\t2147483647\t%s\t%s\n", k, v))
		}
	}

	return writeFileAtomic(path, []byte(b.String()))
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	tmpPath := path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open temp file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
