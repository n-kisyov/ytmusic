package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppStateSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ytmusic", "state.json")

	// Missing file returns defaults without error.
	def, err := LoadAppStateFrom(path)
	if err != nil {
		t.Fatalf("expected nil error for missing state file, got %v", err)
	}
	if def.VideoEnabled {
		t.Fatalf("expected VideoEnabled=false by default")
	}
	if def.CurrentIndex != -1 {
		t.Fatalf("expected CurrentIndex=-1 by default, got %d", def.CurrentIndex)
	}

	state := AppState{
		SearchQuery:       "lofi hip hop",
		ContinuationToken: "token_abc",
		Playlist: []Track{
			{ID: "vid1", Title: "Track One", Artist: "Artist A", DurationSec: 185, DurationText: "3:05", Thumbnail: "https://i.ytimg.com/vi/vid1/hqdefault.jpg"},
			{ID: "vid2", Title: "Track Two", Artist: "Artist B", DurationSec: 210, DurationText: "3:30", Thumbnail: "https://i.ytimg.com/vi/vid2/hqdefault.jpg"},
		},
		Overflow: []Track{
			{ID: "vid3", Title: "Track Three", Artist: "Artist C", DurationSec: 195, DurationText: "3:15"},
		},
		CurrentIndex:     1,
		CurrentTrackID:   "vid2",
		PositionSec:      42.5,
		WasPlaying:       true,
		VideoEnabled:     true,
		PlaylistExpanded: false,
		AlwaysOnTop:      true,
		Theme:            "light",
		Shuffle:          true,
		RepeatMode:       "all",
		LikedTracks: []Track{
			{ID: "vid1", Title: "Track One", Artist: "Artist A", DurationSec: 185, DurationText: "3:05"},
		},
		SavedPlaylists: []SavedPlaylist{
			{
				Name:      "Chill Mix",
				Query:     "lofi hip hop",
				Tracks:    []Track{{ID: "vid1", Title: "Track One", Artist: "Artist A", DurationSec: 185}},
				CreatedAt: 1700000000,
			},
		},
		Volume:  0.65,
		Muted:   false,
		WindowX: 300,
		WindowY: 200,
	}

	if err := SaveAppStateTo(path, state); err != nil {
		t.Fatalf("SaveAppStateTo failed: %v", err)
	}

	loaded, err := LoadAppStateFrom(path)
	if err != nil {
		t.Fatalf("LoadAppStateFrom failed: %v", err)
	}

	if loaded.SearchQuery != state.SearchQuery ||
		loaded.ContinuationToken != state.ContinuationToken ||
		len(loaded.Playlist) != 2 ||
		len(loaded.Overflow) != 1 ||
		loaded.Overflow[0].ID != "vid3" ||
		loaded.CurrentIndex != 1 ||
		loaded.CurrentTrackID != "vid2" ||
		loaded.PositionSec != 42.5 ||
		!loaded.WasPlaying ||
		!loaded.VideoEnabled ||
		loaded.PlaylistExpanded ||
		!loaded.AlwaysOnTop ||
		loaded.Theme != "light" ||
		!loaded.Shuffle ||
		loaded.RepeatMode != "all" ||
		len(loaded.LikedTracks) != 1 ||
		len(loaded.SavedPlaylists) != 1 ||
		loaded.SavedPlaylists[0].Name != "Chill Mix" ||
		loaded.Volume != 0.65 ||
		loaded.WindowX != 300 ||
		loaded.WindowY != 200 {
		t.Fatalf("loaded state mismatch: %+v", loaded)
	}
}

func TestAppStateNormalizationReconcilesTrackID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := AppState{
		Playlist: []Track{
			{ID: "a1", Title: "A"},
			{ID: "b2", Title: "B"},
		},
		CurrentIndex:   0,
		CurrentTrackID: "b2",
		PositionSec:    -5,
		Volume:         2.5,
	}
	if err := SaveAppStateTo(path, state); err != nil {
		t.Fatalf("SaveAppStateTo failed: %v", err)
	}

	loaded, err := LoadAppStateFrom(path)
	if err != nil {
		t.Fatalf("LoadAppStateFrom failed: %v", err)
	}
	if loaded.CurrentIndex != 1 {
		t.Fatalf("expected CurrentIndex reconciled to 1, got %d", loaded.CurrentIndex)
	}
	if loaded.PositionSec != 0 {
		t.Fatalf("expected negative PositionSec clamped to 0, got %v", loaded.PositionSec)
	}
	if loaded.Volume != 0.8 {
		t.Fatalf("expected out-of-range Volume reset to 0.8, got %v", loaded.Volume)
	}
}

func TestAuthStateAndNetscapeCookies(t *testing.T) {
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	cookiesPath := filepath.Join(dir, "cookies.txt")

	auth := AuthState{
		LoggedIn:    true,
		AccountName: "Test User",
		Cookies: map[string]string{
			"SAPISID": "abc123",
			"SID":     "xyz789",
		},
		CookieEntries: []CookieEntry{
			{Name: "SAPISID", Value: "abc123", Domain: ".youtube.com", Path: "/", Secure: true, Expires: 1900000000},
			{Name: "SID", Value: "xyz789", Domain: ".youtube.com", Path: "/", Secure: true, HTTPOnly: true, Expires: 1900000000},
		},
		UpdatedAt: 1700000000,
	}

	if err := SaveAuthStateTo(authPath, auth); err != nil {
		t.Fatalf("SaveAuthStateTo failed: %v", err)
	}
	loaded, err := LoadAuthStateFrom(authPath)
	if err != nil {
		t.Fatalf("LoadAuthStateFrom failed: %v", err)
	}
	if !loaded.LoggedIn || loaded.AccountName != "Test User" || loaded.Cookies["SAPISID"] != "abc123" {
		t.Fatalf("loaded auth mismatch: %+v", loaded)
	}

	if err := WriteNetscapeCookies(cookiesPath, loaded); err != nil {
		t.Fatalf("WriteNetscapeCookies failed: %v", err)
	}
	raw, err := os.ReadFile(cookiesPath)
	if err != nil {
		t.Fatalf("read cookies.txt failed: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "SAPISID\tabc123") || !strings.Contains(text, "SID\txyz789") {
		t.Fatalf("unexpected cookies.txt content:\n%s", text)
	}
}
