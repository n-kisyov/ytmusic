package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var fileWriteMu sync.Mutex

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
	VuStyle           string          `json:"vu_style,omitempty"`
	EQPreset          string          `json:"eq_preset,omitempty"`
	PlaybackRate      float64         `json:"playback_rate,omitempty"`
	VideoQuality      int             `json:"video_quality,omitempty"`
	RecentSearches    []string        `json:"recent_searches,omitempty"`
	TotalPlayTimeSec  int64           `json:"total_play_time_sec,omitempty"`
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

// BrandAccount represents a primary Google channel or YouTube Brand Account under the signed-in session.
type BrandAccount struct {
	Name          string `json:"name"`
	Handle        string `json:"handle,omitempty"`
	Email         string `json:"email,omitempty"`
	AvatarURL     string `json:"avatar_url,omitempty"`
	PageID        string `json:"page_id,omitempty"`
	AuthUser      string `json:"auth_user,omitempty"`
	SubscriberTxt string `json:"subscriber_text,omitempty"`
	Selected      bool   `json:"selected"`
}

// AuthState stores YouTube/Google account login credentials, active channel/Brand Account, and cookies.
type AuthState struct {
	LoggedIn      bool              `json:"logged_in"`
	AccountName   string            `json:"account_name"`
	ChannelHandle string            `json:"channel_handle,omitempty"`
	Email         string            `json:"email"`
	AvatarURL     string            `json:"avatar_url,omitempty"`
	ActivePageID  string            `json:"active_page_id,omitempty"`
	AuthUser      string            `json:"auth_user,omitempty"`
	Accounts      []BrandAccount    `json:"accounts,omitempty"`
	NeedsReauth   bool              `json:"needs_reauth,omitempty"`
	Cookies       map[string]string `json:"cookies"`
	CookieEntries []CookieEntry     `json:"cookie_entries,omitempty"`
	UpdatedAt     int64             `json:"updated_at"`
}

// HasFullSessionCookies reports whether auth contains HttpOnly session cookies (not just document.cookie).
func (a AuthState) HasFullSessionCookies() bool {
	if !a.LoggedIn {
		return false
	}
	if len(a.CookieEntries) > 0 {
		return true
	}
	return a.Cookies["__Secure-3PSID"] != "" ||
		a.Cookies["__Secure-1PSID"] != "" ||
		a.Cookies["LOGIN_INFO"] != "" ||
		(a.Cookies["HSID"] != "" && a.Cookies["SSID"] != "")
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
		VuStyle:          "led",
		EQPreset:         "flat",
		PlaybackRate:     1.0,
		VideoQuality:     720,
		RecentSearches:   []string{},
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
	for i := range s.SavedPlaylists {
		if s.SavedPlaylists[i].Tracks == nil {
			s.SavedPlaylists[i].Tracks = []Track{}
		}
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
	switch s.VuStyle {
	case "led", "bar":
	default:
		s.VuStyle = "led"
	}
	switch s.EQPreset {
	case "flat", "bass", "vocal", "bright":
	default:
		s.EQPreset = "flat"
	}
	if s.PlaybackRate < 0.5 || s.PlaybackRate > 2.0 {
		s.PlaybackRate = 1.0
	}
	switch s.VideoQuality {
	case 144, 240, 360, 480, 720, 1080, 1440, 2160:
	default:
		s.VideoQuality = 720
	}
	if s.RecentSearches == nil {
		s.RecentSearches = []string{}
	}
	if s.Volume < 0 || s.Volume > 1 {
		s.Volume = 0.8
	}
	if s.TotalPlayTimeSec < 0 {
		s.TotalPlayTimeSec = 0
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

func (s *AppState) findSavedPlaylistIndex(name string) int {
	for i := range s.SavedPlaylists {
		if strings.EqualFold(s.SavedPlaylists[i].Name, name) {
			return i
		}
	}
	return -1
}

// CreatePlaylist creates a new named playlist (empty or populated) or merges unique tracks if the name exists.
func (s *AppState) CreatePlaylist(name string, tracks []Track) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("playlist name cannot be empty")
	}
	clean := make([]Track, 0, len(tracks))
	seen := make(map[string]struct{}, len(tracks))
	for _, tr := range tracks {
		if tr.ID == "" {
			continue
		}
		if _, dup := seen[tr.ID]; dup {
			continue
		}
		seen[tr.ID] = struct{}{}
		clean = append(clean, tr)
	}

	if idx := s.findSavedPlaylistIndex(name); idx >= 0 {
		existing := &s.SavedPlaylists[idx]
		have := make(map[string]struct{}, len(existing.Tracks))
		for _, tr := range existing.Tracks {
			if tr.ID != "" {
				have[tr.ID] = struct{}{}
			}
		}
		for _, tr := range clean {
			if _, ok := have[tr.ID]; ok {
				continue
			}
			have[tr.ID] = struct{}{}
			existing.Tracks = append(existing.Tracks, tr)
		}
		return nil
	}

	sp := SavedPlaylist{
		Name:      name,
		Tracks:    clean,
		CreatedAt: time.Now().Unix(),
	}
	s.SavedPlaylists = append([]SavedPlaylist{sp}, s.SavedPlaylists...)
	return nil
}

// AddTrackToPlaylist appends a track to the named playlist (creating it if missing), avoiding duplicate IDs.
func (s *AppState) AddTrackToPlaylist(name string, track Track) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, fmt.Errorf("playlist name cannot be empty")
	}
	if track.ID == "" {
		return false, fmt.Errorf("track id cannot be empty")
	}

	idx := s.findSavedPlaylistIndex(name)
	if idx < 0 {
		sp := SavedPlaylist{
			Name:      name,
			Tracks:    []Track{track},
			CreatedAt: time.Now().Unix(),
		}
		s.SavedPlaylists = append([]SavedPlaylist{sp}, s.SavedPlaylists...)
		return true, nil
	}

	for _, existing := range s.SavedPlaylists[idx].Tracks {
		if existing.ID == track.ID {
			return false, nil
		}
	}
	s.SavedPlaylists[idx].Tracks = append(s.SavedPlaylists[idx].Tracks, track)
	return true, nil
}

// RemoveTrackFromPlaylist removes all entries with trackID from the named playlist.
func (s *AppState) RemoveTrackFromPlaylist(name, trackID string) error {
	name = strings.TrimSpace(name)
	idx := s.findSavedPlaylistIndex(name)
	if idx < 0 {
		return nil
	}
	filtered := make([]Track, 0, len(s.SavedPlaylists[idx].Tracks))
	for _, tr := range s.SavedPlaylists[idx].Tracks {
		if tr.ID != trackID {
			filtered = append(filtered, tr)
		}
	}
	s.SavedPlaylists[idx].Tracks = filtered
	return nil
}

// RenamePlaylist renames an existing saved playlist cleanly without colliding with another playlist.
func (s *AppState) RenamePlaylist(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("playlist name cannot be empty")
	}
	idx := s.findSavedPlaylistIndex(oldName)
	if idx < 0 {
		return fmt.Errorf("playlist %q not found", oldName)
	}
	if conflict := s.findSavedPlaylistIndex(newName); conflict >= 0 && conflict != idx {
		return fmt.Errorf("playlist %q already exists", newName)
	}
	s.SavedPlaylists[idx].Name = newName
	return nil
}

// MoveTrackInPlaylist moves a track within a saved playlist from fromIdx to toIdx.
func (s *AppState) MoveTrackInPlaylist(name string, fromIdx, toIdx int) error {
	idx := s.findSavedPlaylistIndex(strings.TrimSpace(name))
	if idx < 0 {
		return fmt.Errorf("playlist %q not found", name)
	}
	tracks := s.SavedPlaylists[idx].Tracks
	if fromIdx < 0 || fromIdx >= len(tracks) || toIdx < 0 || toIdx >= len(tracks) {
		return fmt.Errorf("track move indices out of range")
	}
	if fromIdx == toIdx {
		return nil
	}
	tr := tracks[fromIdx]
	tracks = append(tracks[:fromIdx], tracks[fromIdx+1:]...)
	if toIdx >= len(tracks) {
		tracks = append(tracks, tr)
	} else {
		tracks = append(tracks[:toIdx], append([]Track{tr}, tracks[toIdx:]...)...)
	}
	s.SavedPlaylists[idx].Tracks = tracks
	return nil
}

// AddRecentSearch records a search query at the front of RecentSearches (case-insensitive dedupe, cap 10).
func (s *AppState) AddRecentSearch(query string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return
	}
	out := make([]string, 0, 10)
	out = append(out, query)
	for _, existing := range s.RecentSearches {
		if len(out) >= 10 {
			break
		}
		if strings.EqualFold(strings.TrimSpace(existing), query) {
			continue
		}
		out = append(out, existing)
	}
	s.RecentSearches = out
}

// ResetPlayTimeStats clears persisted all-time listening time in state.json.
func ResetPlayTimeStats() error {
	path, err := StateFilePath()
	if err != nil {
		return err
	}
	return ResetPlayTimeStatsFrom(path)
}

// ResetPlayTimeStatsFrom clears persisted all-time listening time in the given state file.
func ResetPlayTimeStatsFrom(path string) error {
	state, err := LoadAppStateFrom(path)
	if err != nil {
		return err
	}
	state.TotalPlayTimeSec = 0
	return SaveAppStateTo(path, state)
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
	fileWriteMu.Lock()
	defer fileWriteMu.Unlock()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", dir, err)
	}
	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := f.Name()
	cleanup := func() {
		_ = f.Close()
		_ = os.Remove(tmpPath)
	}
	_ = f.Chmod(0o600)
	if _, err := f.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		cleanup()
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
