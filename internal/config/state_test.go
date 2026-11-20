package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
		LoggedIn:      true,
		AccountName:   "Brand Studio",
		ChannelHandle: "@brandstudio",
		Email:         "owner@example.com",
		AvatarURL:     "https://yt3.ggpht.com/avatar.jpg",
		ActivePageID:  "109876543210987654321",
		AuthUser:      "0",
		Accounts: []BrandAccount{
			{Name: "Personal User", Email: "owner@example.com", Selected: false},
			{Name: "Brand Studio", Handle: "@brandstudio", PageID: "109876543210987654321", Selected: true},
		},
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
	if !loaded.LoggedIn || loaded.AccountName != "Brand Studio" || loaded.ChannelHandle != "@brandstudio" ||
		loaded.ActivePageID != "109876543210987654321" || len(loaded.Accounts) != 2 || !loaded.HasFullSessionCookies() ||
		loaded.Cookies["SAPISID"] != "abc123" {
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

func TestSavedPlaylistCreateAddAndRemoveTrack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := DefaultAppState()
	if err := s.CreatePlaylist("Workout", nil); err != nil {
		t.Fatalf("CreatePlaylist empty failed: %v", err)
	}
	if len(s.SavedPlaylists) != 1 || s.SavedPlaylists[0].Name != "Workout" || len(s.SavedPlaylists[0].Tracks) != 0 {
		t.Fatalf("unexpected created playlist: %+v", s.SavedPlaylists)
	}

	t1 := Track{ID: "t1", Title: "Song 1", Artist: "Artist 1", DurationSec: 180}
	t2 := Track{ID: "t2", Title: "Song 2", Artist: "Artist 2", DurationSec: 200}

	added, err := s.AddTrackToPlaylist("workout", t1)
	if err != nil || !added {
		t.Fatalf("expected t1 added to Workout, got added=%v err=%v", added, err)
	}
	// Duplicate ID should be idempotent.
	addedDup, err := s.AddTrackToPlaylist("Workout", t1)
	if err != nil || addedDup {
		t.Fatalf("expected duplicate t1 skipped, got added=%v err=%v", addedDup, err)
	}
	added2, err := s.AddTrackToPlaylist("Workout", t2)
	if err != nil || !added2 {
		t.Fatalf("expected t2 added to Workout, got added=%v err=%v", added2, err)
	}
	if len(s.SavedPlaylists[0].Tracks) != 2 {
		t.Fatalf("expected 2 tracks in Workout, got %d", len(s.SavedPlaylists[0].Tracks))
	}

	if err := s.RemoveTrackFromPlaylist("Workout", "t1"); err != nil {
		t.Fatalf("RemoveTrackFromPlaylist failed: %v", err)
	}
	if len(s.SavedPlaylists[0].Tracks) != 1 || s.SavedPlaylists[0].Tracks[0].ID != "t2" {
		t.Fatalf("unexpected tracks after removal: %+v", s.SavedPlaylists[0].Tracks)
	}

	if err := SaveAppStateTo(path, s); err != nil {
		t.Fatalf("SaveAppStateTo failed: %v", err)
	}
	loaded, err := LoadAppStateFrom(path)
	if err != nil {
		t.Fatalf("LoadAppStateFrom failed: %v", err)
	}
	if len(loaded.SavedPlaylists) != 1 || len(loaded.SavedPlaylists[0].Tracks) != 1 || loaded.SavedPlaylists[0].Tracks[0].ID != "t2" {
		t.Fatalf("persisted playlist mismatch: %+v", loaded.SavedPlaylists)
	}
}

func TestSaveAppStateToConcurrent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			st := DefaultAppState()
			st.SearchQuery = fmt.Sprintf("query-%d", idx)
			st.PositionSec = float64(idx)
			if err := SaveAppStateTo(path, st); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent SaveAppStateTo failed: %v", err)
	}
	loaded, err := LoadAppStateFrom(path)
	if err != nil {
		t.Fatalf("LoadAppStateFrom after concurrent writes failed: %v", err)
	}
	if !strings.HasPrefix(loaded.SearchQuery, "query-") {
		t.Fatalf("unexpected persisted search query: %q", loaded.SearchQuery)
	}
}

func TestAppStateRenameMoveAndAudioPrefs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s := DefaultAppState()
	t1 := Track{ID: "t1", Title: "One", Artist: "A", DurationSec: 120}
	t2 := Track{ID: "t2", Title: "Two", Artist: "B", DurationSec: 140}
	t3 := Track{ID: "t3", Title: "Three", Artist: "C", DurationSec: 160}
	if err := s.CreatePlaylist("Roadtrip", []Track{t1, t2, t3}); err != nil {
		t.Fatalf("CreatePlaylist failed: %v", err)
	}
	if err := s.RenamePlaylist("Roadtrip", "Late Night"); err != nil {
		t.Fatalf("RenamePlaylist failed: %v", err)
	}
	if s.SavedPlaylists[0].Name != "Late Night" {
		t.Fatalf("expected renamed playlist 'Late Night', got %q", s.SavedPlaylists[0].Name)
	}
	if err := s.MoveTrackInPlaylist("Late Night", 2, 0); err != nil {
		t.Fatalf("MoveTrackInPlaylist failed: %v", err)
	}
	if s.SavedPlaylists[0].Tracks[0].ID != "t3" || s.SavedPlaylists[0].Tracks[2].ID != "t2" {
		t.Fatalf("unexpected track order after move: %+v", s.SavedPlaylists[0].Tracks)
	}

	s.AddRecentSearch("Taylor Swift")
	s.AddRecentSearch("Daft Punk")
	s.AddRecentSearch("taylor swift")
	if len(s.RecentSearches) != 2 || s.RecentSearches[0] != "taylor swift" {
		t.Fatalf("unexpected recent searches after case-insensitive dedupe: %+v", s.RecentSearches)
	}

	s.VuStyle = "bar"
	s.EQPreset = "bass"
	s.PlaybackRate = 1.25
	if err := SaveAppStateTo(path, s); err != nil {
		t.Fatalf("SaveAppStateTo failed: %v", err)
	}
	loaded, err := LoadAppStateFrom(path)
	if err != nil {
		t.Fatalf("LoadAppStateFrom failed: %v", err)
	}
	if loaded.VuStyle != "bar" || loaded.EQPreset != "bass" || loaded.PlaybackRate != 1.25 || len(loaded.RecentSearches) != 2 {
		t.Fatalf("loaded new prefs mismatch: %+v", loaded)
	}
}
