//go:build windows

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	State     config.AppState  `json:"state"`
	Auth      config.AuthState `json:"auth"`
	StreamURL string           `json:"stream_url"`
}

type searchPayload struct {
	Playlist          []config.Track `json:"playlist"`
	ContinuationToken string         `json:"continuation_token"`
}

type loadMorePayload struct {
	Added []config.Track `json:"added"`
}

// App coordinates the UI WebView2 window, Win32 shell/tray, YouTube client, and stream proxy.
type App struct {
	mu          sync.Mutex
	state       config.AppState
	authState   config.AuthState
	ytClient    *youtube.Client
	resolver    *player.Resolver
	server      *player.Server
	wv          webview2.WebView
	winCtrl     *win32.Controller
	authRunning bool
	exited      bool
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
		state:     state,
		authState: authState,
		ytClient:  ytClient,
		resolver:  resolver,
		server:    server,
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
	savedX, savedY := a.state.WindowX, a.state.WindowY
	a.mu.Unlock()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     debug,
		DataPath:  dataDir,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "ytmusic",
			Width:  fixedWidth,
			Height: uint(initialH),
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
			targetH := computeTargetHeight(a.state.VideoEnabled, a.state.PlaylistExpanded)
			snapshot := a.cloneStateLocked()
			a.mu.Unlock()

			_ = config.SaveAppState(snapshot)
			w.Dispatch(func() {
				a.winCtrl.SetClientSize(fixedWidth, targetH, true)
				w.Eval("window.__trayCommand && window.__trayCommand('toggle_video')")
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
				a.winCtrl.SetAlwaysOnTop(enabled)
				w.Eval("window.__trayCommand && window.__trayCommand('toggle_always_on_top')")
			})
		},
		IsAlwaysOnTop: func() bool {
			a.mu.Lock()
			defer a.mu.Unlock()
			return a.state.AlwaysOnTop
		},
		OnExit: func() {
			a.quit()
		},
	})

	a.winCtrl.SetClientSize(fixedWidth, initialH, false)
	a.winCtrl.SetWindowPosition(savedX, savedY)
	a.winCtrl.SetAlwaysOnTop(initialAlwaysOnTop)

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
		if a.state.CurrentIndex >= 0 && a.state.CurrentIndex < len(a.state.Playlist) {
			cur := a.state.Playlist[a.state.CurrentIndex]
			streamURL = a.server.StreamURL(cur.ID)
			if a.state.CurrentIndex+1 < len(a.state.Playlist) {
				a.resolver.Prefetch(a.state.Playlist[a.state.CurrentIndex+1].ID)
			}
		}

		return readyPayload{
			State:     a.cloneStateLocked(),
			Auth:      a.publicAuthState(a.authState),
			StreamURL: streamURL,
		}, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSearch", func(query string) (searchPayload, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		res, err := a.ytClient.SearchInitialPlaylist(ctx, query, initialTracks)
		if err != nil {
			return searchPayload{}, err
		}

		a.mu.Lock()
		a.state.SearchQuery = query
		a.state.Playlist = append([]config.Track(nil), res.Tracks...)
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
		}, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendLoadMore", func() (loadMorePayload, error) {
		a.mu.Lock()
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
		// Re-check against current playlist in case a new search happened concurrently.
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
		a.state.ContinuationToken = res.ContinuationToken
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return loadMorePayload{Added: added}, nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendSelectTrack", func(index int) (string, error) {
		a.mu.Lock()
		if index < 0 || index >= len(a.state.Playlist) {
			a.mu.Unlock()
			return "", fmt.Errorf("track index %d out of range", index)
		}
		tr := a.state.Playlist[index]
		a.state.CurrentIndex = index
		a.state.CurrentTrackID = tr.ID
		a.state.PositionSec = 0
		a.updateTrayTitleLocked()
		if index+1 < len(a.state.Playlist) {
			a.resolver.Prefetch(a.state.Playlist[index+1].ID)
		}
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return a.server.StreamURL(tr.ID), nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendToggleVideo", func(enabled bool) error {
		a.mu.Lock()
		a.state.VideoEnabled = enabled
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

	if err := w.Bind("backendUpdatePlaybackState", func(index int, positionSec float64, isPlaying bool, volume float64, muted bool) error {
		a.mu.Lock()
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
		snapshot := a.cloneStateLocked()
		a.mu.Unlock()

		_ = config.SaveAppState(snapshot)
		return nil
	}); err != nil {
		return err
	}

	if err := w.Bind("backendAuthAction", func(action string) (config.AuthState, error) {
		if action == "logout" {
			_ = config.ClearAuthState(true)
			empty := config.AuthState{Cookies: map[string]string{}}
			a.mu.Lock()
			a.authState = empty
			a.mu.Unlock()
			a.ytClient.SetAuth(empty)
			a.resolver.SetAuth(empty)
			return a.publicAuthState(empty), nil
		}

		a.mu.Lock()
		if a.authRunning {
			cur := a.publicAuthState(a.authState)
			a.mu.Unlock()
			return cur, nil
		}
		a.authRunning = true
		cur := a.publicAuthState(a.authState)
		a.mu.Unlock()

		go func() {
			defer func() {
				a.mu.Lock()
				a.authRunning = false
				a.mu.Unlock()
			}()
			newAuth, err := auth.LaunchLoginHelper()
			if err == nil && newAuth.LoggedIn {
				a.mu.Lock()
				a.authState = newAuth
				a.mu.Unlock()
				a.ytClient.SetAuth(newAuth)
				a.resolver.SetAuth(newAuth)

				pub := a.publicAuthState(newAuth)
				raw, _ := json.Marshal(pub)
				w.Dispatch(func() {
					w.Eval("window.__onAuthUpdated && window.__onAuthUpdated(" + string(raw) + ")")
				})
			}
		}()

		return cur, nil
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

func (a *App) quit() {
	a.saveFinalState()
	if a.winCtrl != nil {
		a.winCtrl.BeginQuit()
	} else if a.wv != nil {
		a.wv.Terminate()
	}
}

func (a *App) saveFinalState() {
	a.mu.Lock()
	if a.exited {
		a.mu.Unlock()
		return
	}
	a.exited = true
	if a.winCtrl != nil {
		x, y := a.winCtrl.GetWindowPosition()
		a.state.WindowX = x
		a.state.WindowY = y
	}
	snapshot := a.cloneStateLocked()
	a.mu.Unlock()

	_ = config.SaveAppState(snapshot)
}

func (a *App) updateTrayTitleLocked() {
	if a.winCtrl == nil {
		return
	}
	tip := "ytmusic"
	if a.state.CurrentIndex >= 0 && a.state.CurrentIndex < len(a.state.Playlist) {
		tip = "ytmusic — " + a.state.Playlist[a.state.CurrentIndex].Title
	}
	a.winCtrl.UpdateTrayTooltip(tip)
}

func (a *App) cloneStateLocked() config.AppState {
	cp := a.state
	cp.Playlist = append([]config.Track(nil), a.state.Playlist...)
	return cp
}

func (a *App) publicAuthState(in config.AuthState) config.AuthState {
	return config.AuthState{
		LoggedIn:    in.LoggedIn,
		AccountName: in.AccountName,
		Email:       in.Email,
		UpdatedAt:   in.UpdatedAt,
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
