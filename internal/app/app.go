//go:build windows

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	webview2 "github.com/jchv/go-webview2"

	"ytmusic/internal/auth"
	"ytmusic/internal/config"
	"ytmusic/internal/player"
	"ytmusic/internal/ui"
	"ytmusic/internal/win32"
	"ytmusic/internal/youtube"
)

const (
	fixedWidth     = 380
	baseHeight     = 172
	videoHeight    = 214
	playlistHeight = 310
	initialTracks  = 30
)

type readyPayload struct {
	State      config.AppState         `json:"state"`
	Auth       config.AuthState        `json:"auth"`
	StreamURL  string                  `json:"stream_url"`
	Stream     *streamSelectionPayload `json:"stream,omitempty"`
	StatsEpoch uint64                  `json:"stats_epoch"`
}

type streamSelectionPayload struct {
	VideoID            string                 `json:"video_id,omitempty"`
	StreamEpoch        uint64                 `json:"stream_epoch,omitempty"`
	AudioURL           string                 `json:"audio_url"`
	VideoURL           string                 `json:"video_url,omitempty"`
	AudioKbps          int                    `json:"audio_kbps"`
	VideoKbps          int                    `json:"video_kbps"`
	RequestedQuality   int                    `json:"requested_quality"`
	ActiveQuality      int                    `json:"active_quality"`
	ProgressiveOnly    bool                   `json:"progressive_only,omitempty"`
	AvailableQualities []player.QualityOption `json:"available_qualities,omitempty"`
}

type searchPayload struct {
	Playlist          []config.Track `json:"playlist"`
	ContinuationToken string         `json:"continuation_token"`
	Label             string         `json:"label,omitempty"`
	RecentSearches    []string       `json:"recent_searches,omitempty"`
}

type loadMorePayload struct {
	Added []config.Track `json:"added"`
}

// App coordinates the UI WebView2 window, Win32 shell/tray, YouTube client, and stream proxy.
type App struct {
	mu               sync.Mutex
	state            config.AppState
	authState        config.AuthState
	ytClient         *youtube.Client
	resolver         *player.Resolver
	server           *player.Server
	wv               webview2.WebView
	winCtrl          *win32.Controller
	authRunning      bool
	exited           bool
	statsEpoch       uint64
	streamEpoch      uint64
	authEpoch        uint64
	queueEpoch       uint64
	sleepMinutes     int
	sleepDeadline    time.Time
	sleepTimer       *time.Timer
	baseTotalPlaySec int64
	sessionPlaySec   float64
	lastTrayPlaySec  int64
	lastPlaySaveTime time.Time
	playActive       bool
	playAnchorTime   time.Time
}

// New initializes application state, YouTube client, stream resolver, and local UI/stream server.
func New() (*App, error) {
	state, err := config.LoadAppState()
	if err != nil {
		state = config.DefaultAppState()
	}
	authState, err := config.LoadAuthState()
	if err != nil {
		authState = config.AuthState{Cookies: map[string]string{}}
	}

	ytClient := youtube.NewClient(nil)
	ytClient.SetAuth(authState)
	ytClient.SetOverflow(state.Overflow)

	resolver := player.NewResolver(nil)
	resolver.SetAuth(authState)

	uiFS, err := ui.AssetsFS()
	if err != nil {
		return nil, fmt.Errorf("load embedded UI assets: %w", err)
	}

	server, err := player.NewServer(resolver, uiFS)
	if err != nil {
		return nil, fmt.Errorf("start local proxy server: %w", err)
	}

	return &App{
		state:            state,
		authState:        authState,
		ytClient:         ytClient,
		resolver:         resolver,
		server:           server,
		baseTotalPlaySec: state.TotalPlayTimeSec,
		lastTrayPlaySec:  -1,
	}, nil
}

// Run launches the WebView2 window, hooks Win32 tray/animations, and enters the message loop.
func (a *App) Run(debug bool) error {
	defer func() {
		a.saveFinalState()
		_ = a.server.Close()
	}()

	dataDir, err := config.WebViewProfileDir()
	if err != nil {
		return err
	}
	_ = os.MkdirAll(dataDir, 0o755)

	a.mu.Lock()
	initialH := computeTargetHeight(a.state.VideoEnabled, a.state.PlaylistExpanded)
	initialAlwaysOnTop := a.state.AlwaysOnTop
	initialTheme := a.state.Theme
	savedX, savedY := a.state.WindowX, a.state.WindowY
	a.mu.Unlock()

	win32.InstallStartupHook(initialTheme != "light", savedX, savedY, fixedWidth, initialH)

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     debug,
		DataPath:  dataDir,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "ytmusic",
			Width:  fixedWidth,
			Height: uint(initialH),
			IconId: 1,
			Center: savedX < 0 || savedY < 0,
		},
	})
	if w == nil {
		return fmt.Errorf("failed to initialize WebView2 window (ensure Microsoft Edge WebView2 Runtime is installed)")
	}
	a.wv = w

	if err := a.bindMethods(w); err != nil {
		return fmt.Errorf("bind RPC methods: %w", err)
	}

	a.winCtrl = win32.NewController(w.Window(), win32.Callbacks{
		OnPlayPause: func() {
			w.Dispatch(func() {
				w.Eval("window.__trayCommand && window.__trayCommand('play_pause')")
			})
		},
		OnNext: func() {
			w.Dispatch(func() {
				w.Eval("window.__trayCommand && window.__trayCommand('next')")
			})
		},
		OnPrev: func() {
			w.Dispatch(func() {
				w.Eval("window.__trayCommand && window.__trayCommand('prev')")
			})
		},
		OnToggleVideo: func() {
			a.mu.Lock()
			a.state.VideoEnabled = !a.state.VideoEnabled
			enabled := a.state.VideoEnabled
			a.streamEpoch++
			targetH := computeTargetHeight(a.state.VideoEnabled, a.state.PlaylistExpanded)
			snapshot := a.cloneStateLocked()
			a.mu.Unlock()

			_ = config.SaveAppState(snapshot)
			w.Dispatch(func() {
				if a.isExited() {
					return
				}
				a.winCtrl.SetClientSize(fixedWidth, targetH, true)
				w.Eval(fmt.Sprintf("window.__trayCommand && window.__trayCommand('toggle_video', %t)", enabled))
			})
		},
		IsVideoEnabled: func() bool {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.state.VideoEnabled
		},
		OnToggleAlwaysOnTop: func() {
			a.mu.Lock()
			a.state.AlwaysOnTop = !a.state.AlwaysOnTop
			enabled := a.state.AlwaysOnTop
			snapshot := a.cloneStateLocked()
			a.mu.Unlock()

			_ = config.SaveAppState(snapshot)
			w.Dispatch(func() {
				if a.isExited() {
					return
				}
				a.winCtrl.SetAlwaysOnTop(enabled)
				w.Eval(fmt.Sprintf("window.__trayCommand && window.__trayCommand('toggle_always_on_top', %t)", enabled))
			})
		},
		IsAlwaysOnTop: func() bool {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.state.AlwaysOnTop
		},
		OnToggleTheme: func() {
			a.mu.Lock()
			if a.state.Theme == "light" {
				a.state.Theme = "dark"
			} else {
				a.state.Theme = "light"
			}
			theme := a.state.Theme
			snapshot := a.cloneStateLocked()
			a.mu.Unlock()

			_ = config.SaveAppState(snapshot)
			w.Dispatch(func() {
				if a.isExited() {
					return
				}
				a.winCtrl.SetDarkMode(theme != "light")
				w.Eval(fmt.Sprintf("window.__trayCommand && window.__trayCommand('toggle_theme', %q)", theme))
			})
		},
		IsLightTheme: func() bool {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.state.Theme == "light"
		},
		OnToggleShuffle: func() {
			a.mu.Lock()
			a.state.Shuffle = !a.state.Shuffle
			enabled := a.state.Shuffle
			snapshot := a.cloneStateLocked()
			a.mu.Unlock()

			_ = config.SaveAppState(snapshot)
			w.Dispatch(func() {
				if a.isExited() {
					return
				}
				w.Eval(fmt.Sprintf("window.__trayCommand && window.__trayCommand('toggle_shuffle', %t)", enabled))
			})
		},
		IsShuffleEnabled: func() bool {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.state.Shuffle
		},
		OnCycleRepeat: func() {
			a.mu.Lock()
			switch a.state.RepeatMode {
			case "off":
				a.state.RepeatMode = "all"
			case "all":
				a.state.RepeatMode = "one"
			default:
				a.state.RepeatMode = "off"
			}
			mode := a.state.RepeatMode
			snapshot := a.cloneStateLocked()
			a.mu.Unlock()

			_ = config.SaveAppState(snapshot)
			w.Dispatch(func() {
				if a.isExited() {
					return
				}
				w.Eval(fmt.Sprintf("window.__trayCommand && window.__trayCommand('cycle_repeat', %q)", mode))
			})
		},
		GetRepeatMode: func() string {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.state.RepeatMode
		},
		OnSetSleepTimer: func(minutes int) {
			a.setSleepTimer(minutes)
		},
		GetSleepTimerMinutes: func() int {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.sleepMinutes
		},
		OnExit: func() {
			a.quit()
		},
		OnSystemSuspend: func() {
			a.handleSystemSuspend(w)
		},
		OnSystemResume: func() {
			a.handleSystemResume(w)
		},
		OnResetStats: func() {
			a.ResetPlayTimeStats()
		},
	})

	a.winCtrl.SetWindowPosition(savedX, savedY)
	a.winCtrl.SetClientSize(fixedWidth, initialH, false)
	a.winCtrl.SetAlwaysOnTop(initialAlwaysOnTop)
	a.winCtrl.SetDarkMode(initialTheme != "light")

	// Fallback reveal in case WebView2 navigation or DOM ready is delayed.
	revealFallback := time.AfterFunc(3000*time.Millisecond, func() {
		w.Dispatch(func() {
			if a.isExited() {
				return
			}
			if a.winCtrl != nil {
				a.winCtrl.RevealWindow()
			}
		})
	})
	defer revealFallback.Stop()

	a.updateTrayTitleLocked()
	w.Navigate(a.server.BaseURL() + "/")
	w.Run()
	return nil
}

// RunSmokeTest verifies state persistence, local stream proxy, and embedded UI without blocking on a GUI loop.
func (a *App) RunSmokeTest() error {
	defer func() {
		_ = a.server.Close()
	}()

	a.mu.Lock()
	snapshot := a.cloneStateLocked()
	a.mu.Unlock()

	if err := config.SaveAppState(snapshot); err != nil {
		return fmt.Errorf("save initial state: %w", err)
	}
	statePath, err := config.StateFilePath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(statePath); err != nil {
		return fmt.Errorf("state file missing after save: %w", err)
	}

	resp, err := http.Get(a.server.BaseURL() + "/")
	if err != nil {
		return fmt.Errorf("fetch embedded UI from proxy: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected UI HTTP status: %d", resp.StatusCode)
	}
	return nil
}

func (a *App) bindMethods(w webview2.WebView) error {
	if err := w.Bind("backendReady", func() (readyPayload, error) {
		a.mu.Lock()
		defer a.mu.Unlock()

		var streamURL string
		var streamInfo *streamSelectionPayload
		if a.state.CurrentIndex >= 0 && a.state.CurrentIndex < len(a.state.Playlist) {
			cur := a.state.Playlist[a.state.CurrentIndex]
			prefQ := a.state.VideoQuality
			if prefQ <= 0 {
				prefQ = 720
			}
			audioURL := a.server.StreamVariantURL(cur.ID, "audio", prefQ)
			streamURL = audioURL
			var videoURL string
			if a.state.VideoEnabled {
				videoURL = a.server.StreamVariantURL(cur.ID, "video", prefQ)
			}
			streamEpoch := a.streamEpoch
			streamInfo = &streamSelectionPayload{
				VideoID:          cur.ID,
				StreamEpoch:      streamEpoch,
				AudioURL:         audioURL,
				VideoURL:         videoURL,
				RequestedQuality: prefQ,
				ActiveQuality:    prefQ,
			}
			if a.state.CurrentIndex+1 < len(a.state.Playlist) {
				a.resolver.Prefetch(a.state.Playlist[a.state.CurrentIndex+1].ID)
			}
			go a.broadcastCurrentStreamInfo(w, cur.ID, streamEpoch)
		}

		needsProfileSync := a.authState.LoggedIn &&
			a.authState.HasFullSessionCookies() &&
			(a.authState.AccountName == "" || a.authState.AccountName == "YouTube Account" || len(a.authState.Accounts) == 0)

		if needsProfileSync {
			go a.refreshAndBroadcastAuth(w)
		}

		return readyPayload{
			State:      a.cloneStateLocked(),
			Auth:       a.publicAuthState(a.authState),
			StreamURL:  streamURL,
			Stream:     streamInfo,
			StatsEpoch: a.statsEpoch,
		}, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSearch", func(query string) (searchPayload, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		var (
			res   youtube.SearchResult
			label string
			err   error
		)
		if _, _, isURL := youtube.ParseYouTubeInput(query); isURL {
			res, label, err = a.ytClient.ResolveURLPlaylist(ctx, query, initialTracks)
		} else {
			res, err = a.ytClient.SearchInitialPlaylist(ctx, query, initialTracks)
		}
		if err != nil {
			return searchPayload{}, err
		}

		a.mu.Lock()
		a.queueEpoch++
		a.streamEpoch++
		a.state.SearchQuery = query
		if _, _, isURL := youtube.ParseYouTubeInput(query); !isURL {
			a.state.AddRecentSearch(query)
		}
		a.state.Playlist = append([]config.Track(nil), res.Tracks...)
		a.state.Overflow = a.ytClient.GetOverflow()
		a.state.ContinuationToken = res.ContinuationToken
		a.state.PositionSec = 0
		if len(a.state.Playlist) > 0 {
			a.state.CurrentIndex = 0
			a.state.CurrentTrackID = a.state.Playlist[0].ID
			a.state.WasPlaying = true
		} else {
			a.state.CurrentIndex = -1
			a.state.CurrentTrackID = ""
			a.state.WasPlaying = false
		}
		recent := append([]string(nil), a.state.RecentSearches...)
		snapshot := a.cloneStateLocked()
		a.updateTrayTitleLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)

		return searchPayload{
			Playlist:          res.Tracks,
			ContinuationToken: res.ContinuationToken,
			Label:             label,
			RecentSearches:    recent,
		}, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendStartRadio", func(seedTrack config.Track) (searchPayload, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		res, err := a.ytClient.StartRadioMix(ctx, seedTrack.ID, &seedTrack, initialTracks)
		if err != nil {
			return searchPayload{}, err
		}

		a.mu.Lock()
		a.queueEpoch++
		a.streamEpoch++
		if seedTrack.Title != "" {
			a.state.SearchQuery = "Mix: " + seedTrack.Title
		}
		a.state.Playlist = append([]config.Track(nil), res.Tracks...)
		a.state.Overflow = a.ytClient.GetOverflow()
		a.state.ContinuationToken = res.ContinuationToken
		a.state.PositionSec = 0
		if len(a.state.Playlist) > 0 {
			a.state.CurrentIndex = 0
			a.state.CurrentTrackID = a.state.Playlist[0].ID
			a.state.WasPlaying = true
		} else {
			a.state.CurrentIndex = -1
			a.state.CurrentTrackID = ""
			a.state.WasPlaying = false
		}
		snapshot := a.cloneStateLocked()
		a.updateTrayTitleLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)

		return searchPayload{
			Playlist:          res.Tracks,
			ContinuationToken: res.ContinuationToken,
			Label:             fmt.Sprintf("Radio Mix (%d songs)", len(res.Tracks)),
		}, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSearchSuggest", func(query string) ([]string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return a.ytClient.SuggestQueries(ctx, query)
	}); err != nil {
		return err
	}

	if err := w.Bind("backendRemoveRecentSearch", func(query string) ([]string, error) {
		a.mu.Lock()
		filtered := make([]string, 0, len(a.state.RecentSearches))
		for _, item := range a.state.RecentSearches {
			if !strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(query)) {
				filtered = append(filtered, item)
			}
		}
		a.state.RecentSearches = filtered
		out := append([]string(nil), filtered...)
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendLoadMore", func() (loadMorePayload, error) {
		a.mu.Lock()
		qEpoch := a.queueEpoch
		token := a.state.ContinuationToken
		existing := make(map[string]struct{}, len(a.state.Playlist))
		var seedID string
		for _, t := range a.state.Playlist {
			existing[t.ID] = struct{}{}
			seedID = t.ID
		}
		a.mu.Unlock()

		if len(existing) == 0 {
			return loadMorePayload{Added: []config.Track{}}, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		res, err := a.ytClient.FetchMoreSongs(ctx, token, seedID, existing)
		if err != nil {
			return loadMorePayload{}, err
		}

		a.mu.Lock()
		if a.queueEpoch != qEpoch {
			a.mu.Unlock()
			return loadMorePayload{Added: []config.Track{}}, nil
		}
		curIDs := make(map[string]struct{}, len(a.state.Playlist))
		for _, t := range a.state.Playlist {
			curIDs[t.ID] = struct{}{}
		}
		added := make([]config.Track, 0, len(res.Tracks))
		for _, t := range res.Tracks {
			if _, ok := curIDs[t.ID]; !ok {
				curIDs[t.ID] = struct{}{}
				added = append(added, t)
			}
		}
		a.state.Playlist = append(a.state.Playlist, added...)
		a.state.Overflow = a.ytClient.GetOverflow()
		a.state.ContinuationToken = res.ContinuationToken
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return loadMorePayload{Added: added}, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSelectTrack", func(index int) (streamSelectionPayload, error) {
		a.mu.Lock()
		if index < 0 || index >= len(a.state.Playlist) {
			a.mu.Unlock()
			return streamSelectionPayload{}, fmt.Errorf("track index %d out of range", index)
		}
		tr := a.state.Playlist[index]
		a.state.CurrentIndex = index
		a.state.CurrentTrackID = tr.ID
		a.state.PositionSec = 0
		a.streamEpoch++
		epoch := a.streamEpoch
		videoEnabled := a.state.VideoEnabled
		prefQ := a.state.VideoQuality
		a.updateTrayTitleLocked()
		if index+1 < len(a.state.Playlist) {
			a.resolver.Prefetch(a.state.Playlist[index+1].ID)
		}
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		out := a.buildStreamSelection(tr.ID, videoEnabled, prefQ)
		out.StreamEpoch = epoch
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendGetCurrentStreamInfo", func() (streamSelectionPayload, error) {
		a.mu.Lock()
		if a.state.CurrentIndex < 0 || a.state.CurrentIndex >= len(a.state.Playlist) {
			a.mu.Unlock()
			return streamSelectionPayload{}, nil
		}
		tr := a.state.Playlist[a.state.CurrentIndex]
		videoEnabled := a.state.VideoEnabled
		prefQ := a.state.VideoQuality
		epoch := a.streamEpoch
		a.mu.Unlock()
		out := a.buildStreamSelection(tr.ID, videoEnabled, prefQ)
		out.StreamEpoch = epoch
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSetVideoQuality", func(quality int) (streamSelectionPayload, error) {
		switch quality {
		case 144, 240, 360, 480, 720, 1080, 1440, 2160:
		default:
			quality = 720
		}
		a.mu.Lock()
		a.state.VideoQuality = quality
		a.streamEpoch++
		epoch := a.streamEpoch
		var trackID string
		if a.state.CurrentIndex >= 0 && a.state.CurrentIndex < len(a.state.Playlist) {
			trackID = a.state.Playlist[a.state.CurrentIndex].ID
		}
		videoEnabled := a.state.VideoEnabled
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		if trackID == "" {
			return streamSelectionPayload{RequestedQuality: quality, ActiveQuality: quality, StreamEpoch: epoch}, nil
		}
		out := a.buildStreamSelection(trackID, videoEnabled, quality)
		out.StreamEpoch = epoch
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendToggleVideo", func(enabled bool) (streamSelectionPayload, error) {
		a.mu.Lock()
		a.state.VideoEnabled = enabled
		a.streamEpoch++
		epoch := a.streamEpoch
		targetH := computeTargetHeight(a.state.VideoEnabled, a.state.PlaylistExpanded)
		var trackID string
		if a.state.CurrentIndex >= 0 && a.state.CurrentIndex < len(a.state.Playlist) {
			trackID = a.state.Playlist[a.state.CurrentIndex].ID
		}
		prefQ := a.state.VideoQuality
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		if a.winCtrl != nil {
			w.Dispatch(func() {
				if a.isExited() {
					return
				}
				a.winCtrl.SetClientSize(fixedWidth, targetH, true)
			})
		}
		if trackID == "" {
			return streamSelectionPayload{RequestedQuality: prefQ, StreamEpoch: epoch}, nil
		}
		out := a.buildStreamSelection(trackID, enabled, prefQ)
		out.StreamEpoch = epoch
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendTogglePlaylist", func(expanded bool) error {
		a.mu.Lock()
		a.state.PlaylistExpanded = expanded
		targetH := computeTargetHeight(a.state.VideoEnabled, a.state.PlaylistExpanded)
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		if a.winCtrl != nil {
			w.Dispatch(func() {
				a.winCtrl.SetClientSize(fixedWidth, targetH, true)
			})
		}
		return nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendToggleAlwaysOnTop", func(enabled bool) error {
		a.mu.Lock()
		a.state.AlwaysOnTop = enabled
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		if a.winCtrl != nil {
			w.Dispatch(func() {
				a.winCtrl.SetAlwaysOnTop(enabled)
			})
		}
		return nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSetTheme", func(theme string) error {
		if theme != "light" {
			theme = "dark"
		}
		a.mu.Lock()
		a.state.Theme = theme
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		if a.winCtrl != nil {
			w.Dispatch(func() {
				a.winCtrl.SetDarkMode(theme != "light")
			})
		}
		return nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendShowWindow", func() error {
		if a.winCtrl != nil {
			w.Dispatch(func() {
				a.winCtrl.RevealWindow()
			})
		}
		return nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSetModes", func(shuffle bool, repeatMode string) error {
		switch repeatMode {
		case "off", "all", "one":
		default:
			repeatMode = "off"
		}
		a.mu.Lock()
		a.state.Shuffle = shuffle
		a.state.RepeatMode = repeatMode
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendUpdatePlaylist", func(playlist []config.Track, currentIndex int) error {
		a.mu.Lock()
		a.queueEpoch++
		a.state.Playlist = append([]config.Track(nil), playlist...)
		if len(a.state.Playlist) == 0 {
			a.ytClient.ClearOverflow()
			a.state.Overflow = nil
			a.state.ContinuationToken = ""
			a.state.PositionSec = 0
			a.state.WasPlaying = false
			a.streamEpoch++
		}
		if currentIndex >= 0 && currentIndex < len(a.state.Playlist) {
			a.state.CurrentIndex = currentIndex
			a.state.CurrentTrackID = a.state.Playlist[currentIndex].ID
		} else {
			a.state.CurrentIndex = -1
			a.state.CurrentTrackID = ""
		}
		a.updateTrayTitleLocked()
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendToggleLike", func(track config.Track) ([]config.Track, error) {
		if track.ID == "" {
			a.mu.Lock()
			liked := append([]config.Track(nil), a.state.LikedTracks...)
			a.mu.Unlock()
			return liked, nil
		}
		a.mu.Lock()
		idx := -1
		for i, t := range a.state.LikedTracks {
			if t.ID == track.ID {
				idx = i
				break
			}
		}
		if idx >= 0 {
			a.state.LikedTracks = append(a.state.LikedTracks[:idx], a.state.LikedTracks[idx+1:]...)
		} else {
			a.state.LikedTracks = append([]config.Track{track}, a.state.LikedTracks...)
		}
		liked := append([]config.Track(nil), a.state.LikedTracks...)
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return liked, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSavePlaylist", func(name string, mode string) ([]config.SavedPlaylist, error) {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("playlist name cannot be empty")
		}
		a.mu.Lock()
		if len(a.state.Playlist) == 0 {
			a.mu.Unlock()
			return nil, fmt.Errorf("current queue is empty")
		}
		existingIdx := -1
		for i, existing := range a.state.SavedPlaylists {
			if strings.EqualFold(existing.Name, name) {
				existingIdx = i
				break
			}
		}
		if existingIdx >= 0 && mode == "merge" {
			if err := a.state.CreatePlaylist(name, a.state.Playlist); err != nil {
				a.mu.Unlock()
				return nil, err
			}
		} else {
			sp := config.SavedPlaylist{
				Name:      name,
				Query:     a.state.SearchQuery,
				Tracks:    append([]config.Track(nil), a.state.Playlist...),
				CreatedAt: time.Now().Unix(),
			}
			if existingIdx >= 0 {
				a.state.SavedPlaylists[existingIdx] = sp
			} else {
				a.state.SavedPlaylists = append([]config.SavedPlaylist{sp}, a.state.SavedPlaylists...)
			}
		}
		out := append([]config.SavedPlaylist(nil), a.state.SavedPlaylists...)
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendLoadLikedPlaylist", func() (searchPayload, error) {
		a.mu.Lock()
		a.queueEpoch++
		a.streamEpoch++
		a.ytClient.ClearOverflow()
		a.state.Overflow = nil
		a.state.ContinuationToken = ""
		a.state.SearchQuery = ""
		a.state.Playlist = append([]config.Track(nil), a.state.LikedTracks...)
		a.state.PositionSec = 0
		if len(a.state.Playlist) > 0 {
			a.state.CurrentIndex = 0
			a.state.CurrentTrackID = a.state.Playlist[0].ID
			a.state.WasPlaying = true
		} else {
			a.state.CurrentIndex = -1
			a.state.CurrentTrackID = ""
			a.state.WasPlaying = false
		}
		a.updateTrayTitleLocked()
		tracks := append([]config.Track(nil), a.state.Playlist...)
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return searchPayload{Playlist: tracks}, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendLoadSavedPlaylist", func(name string) (searchPayload, error) {
		a.mu.Lock()
		var found *config.SavedPlaylist
		for i := range a.state.SavedPlaylists {
			if strings.EqualFold(a.state.SavedPlaylists[i].Name, name) {
				found = &a.state.SavedPlaylists[i]
				break
			}
		}
		if found == nil {
			a.mu.Unlock()
			return searchPayload{}, fmt.Errorf("saved playlist %q not found", name)
		}
		a.queueEpoch++
		a.streamEpoch++
		a.ytClient.ClearOverflow()
		a.state.Overflow = nil
		a.state.SearchQuery = found.Query
		a.state.ContinuationToken = ""
		a.state.Playlist = append([]config.Track(nil), found.Tracks...)
		a.state.PositionSec = 0
		if len(a.state.Playlist) > 0 {
			a.state.CurrentIndex = 0
			a.state.CurrentTrackID = a.state.Playlist[0].ID
			a.state.WasPlaying = true
		} else {
			a.state.CurrentIndex = -1
			a.state.CurrentTrackID = ""
			a.state.WasPlaying = false
		}
		a.updateTrayTitleLocked()
		tracks := append([]config.Track(nil), a.state.Playlist...)
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return searchPayload{Playlist: tracks}, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendDeleteSavedPlaylist", func(name string) ([]config.SavedPlaylist, error) {
		a.mu.Lock()
		filtered := make([]config.SavedPlaylist, 0, len(a.state.SavedPlaylists))
		for _, sp := range a.state.SavedPlaylists {
			if !strings.EqualFold(sp.Name, name) {
				filtered = append(filtered, sp)
			}
		}
		a.state.SavedPlaylists = filtered
		out := append([]config.SavedPlaylist(nil), a.state.SavedPlaylists...)
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendCreatePlaylist", func(name string, tracks []config.Track) ([]config.SavedPlaylist, error) {
		a.mu.Lock()
		if err := a.state.CreatePlaylist(name, tracks); err != nil {
			a.mu.Unlock()
			return nil, err
		}
		snapshot := a.cloneStateLocked()
		out := append([]config.SavedPlaylist(nil), snapshot.SavedPlaylists...)
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendAddTrackToPlaylist", func(name string, track config.Track) ([]config.SavedPlaylist, error) {
		a.mu.Lock()
		if _, err := a.state.AddTrackToPlaylist(name, track); err != nil {
			a.mu.Unlock()
			return nil, err
		}
		snapshot := a.cloneStateLocked()
		out := append([]config.SavedPlaylist(nil), snapshot.SavedPlaylists...)
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendRemoveTrackFromPlaylist", func(name string, trackID string) ([]config.SavedPlaylist, error) {
		a.mu.Lock()
		if err := a.state.RemoveTrackFromPlaylist(name, trackID); err != nil {
			a.mu.Unlock()
			return nil, err
		}
		snapshot := a.cloneStateLocked()
		out := append([]config.SavedPlaylist(nil), snapshot.SavedPlaylists...)
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendRenamePlaylist", func(oldName, newName string) ([]config.SavedPlaylist, error) {
		a.mu.Lock()
		if err := a.state.RenamePlaylist(oldName, newName); err != nil {
			a.mu.Unlock()
			return nil, err
		}
		snapshot := a.cloneStateLocked()
		out := append([]config.SavedPlaylist(nil), snapshot.SavedPlaylists...)
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendMovePlaylistTrack", func(name string, fromIdx, toIdx int) ([]config.SavedPlaylist, error) {
		a.mu.Lock()
		if err := a.state.MoveTrackInPlaylist(name, fromIdx, toIdx); err != nil {
			a.mu.Unlock()
			return nil, err
		}
		snapshot := a.cloneStateLocked()
		out := append([]config.SavedPlaylist(nil), snapshot.SavedPlaylists...)
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return out, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSetAudioPrefs", func(vuStyle, eqPreset string, playbackRate float64) error {
		a.mu.Lock()
		if vuStyle != "" {
			a.state.VuStyle = vuStyle
		}
		if eqPreset != "" {
			a.state.EQPreset = eqPreset
		}
		if playbackRate >= 0.5 && playbackRate <= 2.0 {
			a.state.PlaybackRate = playbackRate
		}
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSetSleepTimer", func(minutes int) (int64, error) {
		a.setSleepTimer(minutes)
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.sleepDeadline.IsZero() {
			return 0, nil
		}
		rem := int64(time.Until(a.sleepDeadline).Seconds())
		if rem < 0 {
			rem = 0
		}
		return rem, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendUpdatePlaybackState", func(index int, positionSec float64, isPlaying bool, volume float64, muted bool, sessionPlaySec float64, epoch uint64) error {
		a.mu.Lock()
		stateChanged := a.state.CurrentIndex != index || a.state.WasPlaying != isPlaying || a.state.Volume != volume || a.state.Muted != muted
		if index >= -1 && index < len(a.state.Playlist) {
			a.state.CurrentIndex = index
			if index >= 0 {
				a.state.CurrentTrackID = a.state.Playlist[index].ID
			} else {
				a.state.CurrentTrackID = ""
			}
		}
		if positionSec >= 0 {
			a.state.PositionSec = positionSec
		}
		a.state.WasPlaying = isPlaying
		if volume >= 0 && volume <= 1 {
			a.state.Volume = volume
		}
		a.state.Muted = muted
		if epoch == a.statsEpoch && sessionPlaySec > a.sessionPlaySec {
			a.sessionPlaySec = sessionPlaySec
			a.state.TotalPlayTimeSec = a.baseTotalPlaySec + int64(a.sessionPlaySec)
		}
		a.playActive = isPlaying
		if isPlaying {
			a.playAnchorTime = time.Now()
		} else {
			a.playAnchorTime = time.Time{}
		}
		curSessSec := int64(a.sessionPlaySec)
		if curSessSec != a.lastTrayPlaySec {
			a.lastTrayPlaySec = curSessSec
			a.updateTrayTitleLocked()
		}
		shouldSave := stateChanged || !isPlaying || time.Since(a.lastPlaySaveTime) >= 3*time.Second
		var snapshot config.AppState
		if shouldSave {
			a.lastPlaySaveTime = time.Now()
			snapshot = a.cloneStateLocked()
		}
		a.mu.Unlock()

		if shouldSave {
			_ = config.SaveAppState(snapshot)
		}
		return nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendAuthAction", func(action string) (config.AuthState, error) {
		switch action {
		case "logout":
			empty := config.AuthState{Cookies: map[string]string{}}
			a.mu.Lock()
			a.authEpoch++
			running := a.authRunning
			a.authState = empty
			a.mu.Unlock()
			_ = config.ClearAuthState(!running)
			a.ytClient.SetAuth(empty)
			a.resolver.SetAuth(empty)
			return a.publicAuthState(empty), nil

		case "refresh":
			if err := a.refreshAccountProfileSync(); err != nil {
				return config.AuthState{}, err
			}
			a.mu.Lock()
			cur := a.publicAuthState(a.authState)
			a.mu.Unlock()
			return cur, nil
		}

		a.mu.Lock()
		if a.authRunning {
			cur := a.publicAuthState(a.authState)
			a.mu.Unlock()
			return cur, nil
		}
		a.authRunning = true
		loginEpoch := a.authEpoch
		prevUpdatedAt := a.authState.UpdatedAt
		cur := a.publicAuthState(a.authState)
		a.mu.Unlock()

		go func() {
			defer func() {
				a.mu.Lock()
				a.authRunning = false
				a.mu.Unlock()
			}()
			if action == "switch_google" {
				if dir, dirErr := config.AuthWebViewProfileDir(); dirErr == nil {
					_ = os.RemoveAll(dir)
				}
			}
			newAuth, err := auth.LaunchLoginHelper()
			if err == nil && newAuth.LoggedIn && newAuth.UpdatedAt != prevUpdatedAt {
				a.mu.Lock()
				if a.authEpoch != loginEpoch || a.exited {
					a.mu.Unlock()
					_ = config.ClearAuthState(false)
					return
				}
				a.authEpoch++
				a.authState = newAuth
				a.mu.Unlock()
				a.ytClient.SetAuth(newAuth)
				a.resolver.SetAuth(newAuth)

				pub := a.publicAuthState(newAuth)
				raw, _ := json.Marshal(pub)
				w.Dispatch(func() {
					if a.isExited() {
						return
					}
					w.Eval("window.__onAuthUpdated && window.__onAuthUpdated(" + string(raw) + ", false)")
				})
			}
		}()

		return cur, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSwitchBrandAccount", func(pageID string, authUser string) (config.AuthState, error) {
		a.mu.Lock()
		if !a.authState.LoggedIn {
			a.mu.Unlock()
			return config.AuthState{}, fmt.Errorf("not signed in")
		}
		a.authEpoch++
		a.authState.ActivePageID = pageID
		if authUser != "" {
			a.authState.AuthUser = authUser
		}
		for i := range a.authState.Accounts {
			acc := &a.authState.Accounts[i]
			acc.Selected = acc.PageID == pageID
			if acc.Selected {
				if acc.Name != "" {
					a.authState.AccountName = acc.Name
				}
				a.authState.ChannelHandle = acc.Handle
				a.authState.AvatarURL = acc.AvatarURL
				if acc.Email != "" {
					a.authState.Email = acc.Email
				}
			}
		}
		a.authState.UpdatedAt = time.Now().Unix()
		updatedAuth := a.authState
		a.mu.Unlock()

		a.ytClient.SetAuth(updatedAuth)
		a.resolver.SetAuth(updatedAuth)
		_ = a.refreshAccountProfileSync()

		a.mu.Lock()
		finalAuth := a.authState
		pub := a.publicAuthState(finalAuth)
		a.mu.Unlock()

		_ = config.SaveAuthState(finalAuth)
		return pub, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendOpenExternal", func(rawURL string) error {
		return win32.OpenExternalURL(rawURL)
	}); err != nil {
		return err
	}

	if err := w.Bind("backendQuitApp", func() error {
		w.Dispatch(func() {
			a.quit()
		})
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (a *App) setSleepTimer(minutes int) {
	a.mu.Lock()
	if a.sleepTimer != nil {
		a.sleepTimer.Stop()
		a.sleepTimer = nil
	}
	a.sleepMinutes = minutes
	var remainingSec int64
	wv := a.wv
	if minutes > 0 {
		duration := time.Duration(minutes) * time.Minute
		a.sleepDeadline = time.Now().Add(duration)
		remainingSec = int64(duration.Seconds())
		var timer *time.Timer
		timer = time.AfterFunc(duration, func() {
			a.mu.Lock()
			if a.sleepTimer != timer {
				a.mu.Unlock()
				return
			}
			a.sleepMinutes = 0
			a.sleepDeadline = time.Time{}
			a.sleepTimer = nil
			w := a.wv
			a.mu.Unlock()
			if w != nil {
				w.Dispatch(func() {
					if a.isExited() {
						return
					}
					w.Eval("window.__onSleepTimerChanged && window.__onSleepTimerChanged(0)")
					w.Eval("window.__trayCommand && window.__trayCommand('pause_sleep')")
				})
			}
		})
		a.sleepTimer = timer
	} else {
		a.sleepDeadline = time.Time{}
	}
	a.mu.Unlock()

	if wv != nil {
		msg := "Sleep timer off"
		if minutes > 0 {
			msg = fmt.Sprintf("Sleep timer set for %dm", minutes)
		}
		rawMsg, _ := json.Marshal(msg)
		wv.Dispatch(func() {
			if a.isExited() {
				return
			}
			wv.Eval(fmt.Sprintf("window.__onSleepTimerChanged && window.__onSleepTimerChanged(%d)", remainingSec))
			wv.Eval("window.__showToast && window.__showToast(" + string(rawMsg) + ")")
		})
	}
}

func (a *App) quit() {
	a.mu.Lock()
	if a.sleepTimer != nil {
		a.sleepTimer.Stop()
		a.sleepTimer = nil
	}
	a.mu.Unlock()

	a.saveFinalState()
	if a.winCtrl != nil {
		a.winCtrl.BeginQuit()
	} else if a.wv != nil {
		a.wv.Terminate()
	}
}

func (a *App) creditTrailingPlayTimeLocked() {
	if !a.playActive || a.playAnchorTime.IsZero() {
		return
	}
	now := time.Now()
	delta := now.Sub(a.playAnchorTime).Seconds()
	if delta > 0 && delta < 3600 {
		a.sessionPlaySec += delta
		a.state.TotalPlayTimeSec = a.baseTotalPlaySec + int64(a.sessionPlaySec)
	}
	a.playAnchorTime = now
}

func (a *App) handleSystemSuspend(w webview2.WebView) {
	a.mu.Lock()
	a.creditTrailingPlayTimeLocked()
	a.playActive = false
	a.playAnchorTime = time.Time{}
	a.updateTrayTitleLocked()
	snapshot := a.cloneStateLocked()
	a.mu.Unlock()

	_ = config.SaveAppState(snapshot)
	if w != nil {
		w.Dispatch(func() {
			if a.isExited() {
				return
			}
			w.Eval("window.__trayCommand && window.__trayCommand('system_suspend')")
		})
	}
}

func (a *App) handleSystemResume(w webview2.WebView) {
	a.mu.Lock()
	if a.playActive {
		a.playAnchorTime = time.Now()
	}
	a.mu.Unlock()
	if w != nil {
		w.Dispatch(func() {
			if a.isExited() {
				return
			}
			w.Eval("window.__trayCommand && window.__trayCommand('system_resume')")
		})
	}
}

// ResetPlayTimeStats clears both session and all-time listening counters cleanly.
func (a *App) ResetPlayTimeStats() {
	a.mu.Lock()
	a.statsEpoch++
	epoch := a.statsEpoch
	a.baseTotalPlaySec = 0
	a.sessionPlaySec = 0
	a.lastTrayPlaySec = -1
	if a.playActive {
		a.playAnchorTime = time.Now()
	} else {
		a.playAnchorTime = time.Time{}
	}
	a.state.TotalPlayTimeSec = 0
	a.updateTrayTitleLocked()
	snapshot := a.cloneStateLocked()
	w := a.wv
	a.mu.Unlock()

	_ = config.SaveAppState(snapshot)
	if w != nil {
		w.Dispatch(func() {
			if a.isExited() {
				return
			}
			w.Eval(fmt.Sprintf("window.__onStatsReset && window.__onStatsReset(%d)", epoch))
		})
	}
}

func (a *App) isExited() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.exited
}

func (a *App) saveFinalState() {
	a.mu.Lock()
	if a.exited {
		a.mu.Unlock()
		return
	}
	a.exited = true
	a.creditTrailingPlayTimeLocked()
	a.updateTrayTitleLocked()
	if a.winCtrl != nil {
		x, y := a.winCtrl.GetWindowPosition()
		a.state.WindowX = x
		a.state.WindowY = y
	}
	snapshot := a.cloneStateLocked()
	a.mu.Unlock()

	_ = config.SaveAppState(snapshot)
}

func formatPlayTimeTooltip(sec int64) string {
	if sec < 0 {
		sec = 0
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func (a *App) updateTrayTitleLocked() {
	if a.winCtrl == nil {
		return
	}
	sessSec := int64(a.sessionPlaySec)
	totalSec := a.state.TotalPlayTimeSec
	if totalSec < a.baseTotalPlaySec+sessSec {
		totalSec = a.baseTotalPlaySec + sessSec
	}
	stats := fmt.Sprintf("\nSession: %s | Total: %s", formatPlayTimeTooltip(sessSec), formatPlayTimeTooltip(totalSec))
	tip := "ytmusic" + stats
	if a.state.CurrentIndex >= 0 && a.state.CurrentIndex < len(a.state.Playlist) {
		const prefix = "ytmusic :: "
		maxTitleUnits := 120 - win32.UTF16Len(prefix) - win32.UTF16Len(stats)
		title := win32.TruncateTooltip(a.state.Playlist[a.state.CurrentIndex].Title, maxTitleUnits)
		tip = prefix + title + stats
	}
	a.winCtrl.UpdateTrayTooltip(tip)
}

func (a *App) cloneStateLocked() config.AppState {
	cp := a.state
	cp.Playlist = append([]config.Track(nil), a.state.Playlist...)
	cp.Overflow = append([]config.Track(nil), a.state.Overflow...)
	cp.LikedTracks = append([]config.Track(nil), a.state.LikedTracks...)
	cp.RecentSearches = append([]string(nil), a.state.RecentSearches...)
	cp.SavedPlaylists = make([]config.SavedPlaylist, len(a.state.SavedPlaylists))
	for i, sp := range a.state.SavedPlaylists {
		cp.SavedPlaylists[i] = sp
		cp.SavedPlaylists[i].Tracks = append([]config.Track(nil), sp.Tracks...)
	}
	return cp
}

func (a *App) refreshAccountProfileSync() error {
	a.mu.Lock()
	cur := a.authState
	epoch := a.authEpoch
	a.mu.Unlock()
	if !cur.LoggedIn || !cur.HasFullSessionCookies() {
		return errors.New("sign in with YouTube session cookies first")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	prof, err := a.ytClient.FetchAccountProfile(ctx)
	if err != nil {
		return err
	}

	a.mu.Lock()
	if !a.authState.LoggedIn || a.authEpoch != epoch {
		a.mu.Unlock()
		return errors.New("account state changed during refresh")
	}
	auth.ApplyProfileToAuthState(&a.authState, prof)
	updated := a.authState
	a.mu.Unlock()

	a.ytClient.SetAuth(updated)
	a.resolver.SetAuth(updated)
	_ = config.SaveAuthState(updated)
	return nil
}

func (a *App) refreshAndBroadcastAuth(w webview2.WebView) {
	if err := a.refreshAccountProfileSync(); err != nil || w == nil {
		return
	}
	a.mu.Lock()
	if a.exited || !a.authState.LoggedIn {
		a.mu.Unlock()
		return
	}
	pub := a.publicAuthState(a.authState)
	a.mu.Unlock()
	raw, _ := json.Marshal(pub)
	w.Dispatch(func() {
		if a.isExited() {
			return
		}
		w.Eval("window.__onAuthUpdated && window.__onAuthUpdated(" + string(raw) + ", true)")
	})
}

func (a *App) publicAuthState(in config.AuthState) config.AuthState {
	needsReauth := in.NeedsReauth
	if in.LoggedIn && !in.HasFullSessionCookies() {
		needsReauth = true
	}
	accounts := append([]config.BrandAccount(nil), in.Accounts...)
	return config.AuthState{
		LoggedIn:      in.LoggedIn,
		AccountName:   in.AccountName,
		ChannelHandle: in.ChannelHandle,
		Email:         in.Email,
		AvatarURL:     in.AvatarURL,
		ActivePageID:  in.ActivePageID,
		AuthUser:      in.AuthUser,
		Accounts:      accounts,
		NeedsReauth:   needsReauth,
		UpdatedAt:     in.UpdatedAt,
	}
}

func computeTargetHeight(videoEnabled, playlistExpanded bool) int {
	h := baseHeight
	if videoEnabled {
		h += videoHeight
	}
	if playlistExpanded {
		h += playlistHeight
	}
	return h
}

func (a *App) buildStreamSelection(videoID string, videoEnabled bool, prefQuality int) streamSelectionPayload {
	if prefQuality <= 0 {
		prefQuality = 720
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	sel, err := a.resolver.ResolveCatalogSelection(ctx, videoID, videoEnabled, prefQuality)
	if err != nil {
		var vidURL string
		if videoEnabled {
			vidURL = a.server.StreamVariantURL(videoID, "video", prefQuality)
		}
		return streamSelectionPayload{
			VideoID:          videoID,
			AudioURL:         a.server.StreamVariantURL(videoID, "audio", prefQuality),
			VideoURL:         vidURL,
			RequestedQuality: prefQuality,
			ActiveQuality:    prefQuality,
		}
	}

	return selectionToPayload(a.server, videoID, videoEnabled, sel)
}

func selectionToPayload(srv *player.Server, videoID string, videoEnabled bool, sel player.StreamSelection) streamSelectionPayload {
	out := streamSelectionPayload{
		VideoID:            videoID,
		AudioKbps:          sel.AudioBitrateKbps,
		VideoKbps:          sel.VideoBitrateKbps,
		RequestedQuality:   sel.RequestedQuality,
		ActiveQuality:      sel.ActiveQuality,
		ProgressiveOnly:    sel.ProgressiveOnly,
		AvailableQualities: sel.AvailableQualities,
	}
	if srv != nil {
		if sel.ProgressiveOnly {
			out.AudioURL = srv.StreamURL(videoID)
		} else {
			out.AudioURL = srv.StreamVariantURL(videoID, "audio", sel.RequestedQuality)
			if videoEnabled && sel.VideoURL != "" {
				q := sel.ActiveQuality
				if q <= 0 {
					q = sel.RequestedQuality
				}
				out.VideoURL = srv.StreamVariantURL(videoID, "video", q)
			}
		}
	}
	return out
}

func (a *App) broadcastCurrentStreamInfo(w webview2.WebView, videoID string, expectedEpoch uint64) {
	if w == nil || videoID == "" {
		return
	}
	a.mu.Lock()
	videoEnabled := a.state.VideoEnabled
	prefQ := a.state.VideoQuality
	a.mu.Unlock()

	info := a.buildStreamSelection(videoID, videoEnabled, prefQ)

	// Confirm the active track, video mode, and quality preference didn't change while resolving.
	a.mu.Lock()
	var currentID string
	if a.state.CurrentIndex >= 0 && a.state.CurrentIndex < len(a.state.Playlist) {
		currentID = a.state.Playlist[a.state.CurrentIndex].ID
	}
	curEpoch := a.streamEpoch
	exited := a.exited
	a.mu.Unlock()
	if exited || currentID != videoID || curEpoch != expectedEpoch {
		return
	}

	info.StreamEpoch = expectedEpoch
	raw, _ := json.Marshal(info)
	w.Dispatch(func() {
		a.mu.Lock()
		stillValid := !a.exited && a.streamEpoch == expectedEpoch
		a.mu.Unlock()
		if !stillValid {
			return
		}
		w.Eval("window.__onStreamInfoUpdated && window.__onStreamInfoUpdated(" + string(raw) + ")")
	})
}
