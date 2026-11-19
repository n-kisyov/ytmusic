package player

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"ytmusic/internal/config"
)

const (
	streamCacheTTL    = 90 * time.Minute
	androidUserAgent  = "com.google.android.youtube/20.10.38 (Linux; U; Android 14) gzip"
	desktopChromeUA   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// ResolvedStream holds a resolved stream URL and the User-Agent required by upstream.
type ResolvedStream struct {
	URL       string
	UserAgent string
}

type cachedStream struct {
	URL       string
	UserAgent string
	ExpiresAt time.Time
}

type inflightResolve struct {
	done   chan struct{}
	stream ResolvedStream
	err    error
}

// Resolver resolves playable progressive MP4 (audio+video) stream URLs for YouTube video IDs.
type Resolver struct {
	httpClient *http.Client

	mu       sync.Mutex
	cache    map[string]cachedStream
	inflight map[string]*inflightResolve
	ytdlp    string
	checked  bool
	auth     config.AuthState
}

// NewResolver creates a stream resolver with in-memory caching.
func NewResolver(httpClient *http.Client) *Resolver {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Resolver{
		httpClient: httpClient,
		cache:      make(map[string]cachedStream),
		inflight:   make(map[string]*inflightResolve),
	}
}

// SetAuth updates auth state (cookies.txt is read when LoggedIn is true).
func (r *Resolver) SetAuth(auth config.AuthState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auth = auth
}

// Invalidate removes a cached stream URL (e.g. after upstream 403/410).
func (r *Resolver) Invalidate(videoID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, videoID)
}

// Prefetch warms the stream cache in the background for the upcoming track.
func (r *Resolver) Prefetch(videoID string) {
	if videoID == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, _ = r.Resolve(ctx, videoID)
	}()
}

// Resolve returns a direct CDN URL for the given YouTube video ID.
func (r *Resolver) Resolve(ctx context.Context, videoID string) (string, error) {
	st, err := r.ResolveStream(ctx, videoID)
	return st.URL, err
}

// ResolveStream returns a direct CDN URL and associated User-Agent for the given YouTube video ID.
func (r *Resolver) ResolveStream(ctx context.Context, videoID string) (ResolvedStream, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return ResolvedStream{}, errors.New("video ID is empty")
	}

	r.mu.Lock()
	if entry, ok := r.cache[videoID]; ok && time.Now().Before(entry.ExpiresAt) {
		r.mu.Unlock()
		return ResolvedStream{URL: entry.URL, UserAgent: entry.UserAgent}, nil
	}
	call, ok := r.inflight[videoID]
	if !ok {
		call = &inflightResolve{done: make(chan struct{})}
		r.inflight[videoID] = call
		go func() {
			// Run upstream resolution with a detached timeout so canceling one speculative
			// browser range request does not abort concurrent waiters for the same track.
			resolveCtx, resolveCancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer resolveCancel()

			st, err := r.resolveUncached(resolveCtx, videoID)

			r.mu.Lock()
			if err == nil && st.URL != "" {
				r.cache[videoID] = cachedStream{
					URL:       st.URL,
					UserAgent: st.UserAgent,
					ExpiresAt: time.Now().Add(streamCacheTTL),
				}
			}
			delete(r.inflight, videoID)
			call.stream = st
			call.err = err
			close(call.done)
			r.mu.Unlock()
		}()
	}
	r.mu.Unlock()

	select {
	case <-call.done:
		return call.stream, call.err
	case <-ctx.Done():
		return ResolvedStream{}, ctx.Err()
	}
}

func (r *Resolver) resolveUncached(ctx context.Context, videoID string) (ResolvedStream, error) {
	if st, err := r.resolveWithInnerTube(ctx, videoID); err == nil && st.URL != "" {
		return st, nil
	}
	if bin := r.getYtDlpPath(); bin != "" {
		if st, err := r.resolveWithYtDlp(ctx, bin, videoID); err == nil && st.URL != "" {
			return st, nil
		}
	}
	return ResolvedStream{}, fmt.Errorf("unable to resolve playable stream for %s", videoID)
}

func (r *Resolver) getYtDlpPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.checked {
		return r.ytdlp
	}
	r.checked = true
	r.ytdlp = FindYtDlp()
	return r.ytdlp
}

// FindYtDlp locates yt-dlp.exe in PATH or WinGet install locations.
func FindYtDlp() string {
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p
	}
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		wingetLink := filepath.Join(localAppData, "Microsoft", "WinGet", "Links", "yt-dlp.exe")
		if _, err := os.Stat(wingetLink); err == nil {
			return wingetLink
		}
		matches, _ := filepath.Glob(filepath.Join(localAppData, "Microsoft", "WinGet", "Packages", "yt-dlp*", "yt-dlp.exe"))
		if len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}

func (r *Resolver) resolveWithYtDlp(ctx context.Context, bin, videoID string) (ResolvedStream, error) {
	args := []string{
		"-g",
		"-f", "18/best[ext=mp4][acodec!=none][vcodec!=none]/best[acodec!=none]",
		"--extractor-args", "youtube:player_client=android,web_safari",
		"--no-playlist",
		"--no-warnings",
		"--socket-timeout", "10",
	}

	r.mu.Lock()
	loggedIn := r.auth.LoggedIn
	r.mu.Unlock()

	if loggedIn {
		if cookiesPath, err := config.CookiesTxtPath(); err == nil {
			if _, err := os.Stat(cookiesPath); err == nil {
				args = append(args, "--cookies", cookiesPath)
			}
		}
	}
	args = append(args, "https://www.youtube.com/watch?v="+videoID)

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return ResolvedStream{}, fmt.Errorf("yt-dlp failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			ua := desktopChromeUA
			if strings.Contains(line, "c=ANDROID") {
				ua = androidUserAgent
			}
			return ResolvedStream{URL: line, UserAgent: ua}, nil
		}
	}
	return ResolvedStream{}, errors.New("yt-dlp returned no stream URL")
}

func (r *Resolver) resolveWithInnerTube(ctx context.Context, videoID string) (ResolvedStream, error) {
	type clientDef struct {
		ctx       map[string]any
		userAgent string
	}
	clients := []clientDef{
		{
			ctx: map[string]any{
				"clientName":        "ANDROID",
				"clientVersion":     "20.10.38",
				"androidSdkVersion": 34,
				"osName":            "Android",
				"osVersion":         "14",
				"hl":                "en",
				"gl":                "US",
				"utcOffsetMinutes":  0,
			},
			userAgent: androidUserAgent,
		},
		{
			ctx: map[string]any{
				"clientName":       "IOS",
				"clientVersion":    "19.45.4",
				"deviceMake":       "Apple",
				"deviceModel":      "iPhone16,2",
				"osName":           "iPhone",
				"osVersion":        "18.1.0.22B83",
				"hl":               "en",
				"gl":               "US",
				"utcOffsetMinutes": 0,
			},
			userAgent: "com.google.ios.youtube/19.45.4 (iPhone16,2; U; CPU iOS 18_1_0 like Mac OS X;)",
		},
	}

	var lastErr error
	for _, cl := range clients {
		payload := map[string]any{
			"context": map[string]any{
				"client": cl.ctx,
			},
			"videoId":        videoID,
			"contentCheckOk": true,
			"racyCheckOk":    true,
		}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.youtube.com/youtubei/v1/player?prettyPrint=false", bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", cl.userAgent)

		resp, err := r.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("player API HTTP %d", resp.StatusCode)
			continue
		}

		var data struct {
			StreamingData struct {
				Formats []struct {
					Itag          int    `json:"itag"`
					URL           string `json:"url"`
					MimeType      string `json:"mimeType"`
					AudioChannels int    `json:"audioChannels"`
				} `json:"formats"`
				AdaptiveFormats []struct {
					Itag     int    `json:"itag"`
					URL      string `json:"url"`
					MimeType string `json:"mimeType"`
				} `json:"adaptiveFormats"`
			} `json:"streamingData"`
		}
		if err := json.Unmarshal(respBytes, &data); err != nil {
			lastErr = err
			continue
		}

		candidates := make([]string, 0, 8)
		for _, f := range data.StreamingData.Formats {
			if f.URL != "" && f.Itag == 18 {
				candidates = append(candidates, f.URL)
			}
		}
		for _, f := range data.StreamingData.Formats {
			if f.URL != "" && f.Itag != 18 && strings.Contains(f.MimeType, "video/mp4") {
				candidates = append(candidates, f.URL)
			}
		}
		for _, f := range data.StreamingData.Formats {
			if f.URL != "" && !strings.Contains(f.MimeType, "video/mp4") {
				candidates = append(candidates, f.URL)
			}
		}
		for _, f := range data.StreamingData.AdaptiveFormats {
			if f.URL != "" && strings.HasPrefix(f.MimeType, "audio/mp4") {
				candidates = append(candidates, f.URL)
			}
		}

		for _, u := range candidates {
			if r.probeStreamURL(ctx, u, cl.userAgent) {
				return ResolvedStream{URL: u, UserAgent: cl.userAgent}, nil
			}
		}
		lastErr = errors.New("no playable format URL passed probe")
	}
	if lastErr == nil {
		lastErr = errors.New("failed to resolve stream URL")
	}
	return ResolvedStream{}, lastErr
}

func (r *Resolver) probeStreamURL(ctx context.Context, streamURL, ua string) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, streamURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Range", "bytes=0-1")
	req.Header.Set("User-Agent", ua)
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent
}
