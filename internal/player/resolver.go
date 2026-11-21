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
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"ytmusic/internal/config"
)

const (
	streamCacheTTL   = 90 * time.Minute
	defaultQuality   = 720
	androidUserAgent = "com.google.android.youtube/20.10.38 (Linux; U; Android 14) gzip"
	iosUserAgent     = "com.google.ios.youtube/19.45.4 (iPhone16,2; U; CPU iOS 18_1_0 like Mac OS X;)"
	desktopChromeUA  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// ResolvedStream holds a resolved stream URL and the User-Agent required by upstream.
type ResolvedStream struct {
	URL       string
	UserAgent string
}

// FormatStream describes an individual playable audio, video, or progressive media stream.
type FormatStream struct {
	Itag         int    `json:"itag"`
	URL          string `json:"url"`
	UserAgent    string `json:"-"`
	Kind         string `json:"kind"` // "audio" | "video" | "progressive"
	MimeType     string `json:"mime_type"`
	Height       int    `json:"height,omitempty"`
	QualityLabel string `json:"quality_label,omitempty"`
	BitrateKbps  int    `json:"bitrate_kbps"`
	IsH264       bool   `json:"is_h264,omitempty"`
	IsAAC        bool   `json:"is_aac,omitempty"`
}

// QualityOption represents a selectable video resolution tier and its video bitrate.
type QualityOption struct {
	Height int    `json:"height"`
	Label  string `json:"label"`
	Kbps   int    `json:"kbps"`
}

// TrackCatalog contains all resolved audio, adaptive video, and progressive streams for a track.
type TrackCatalog struct {
	AudioFormats []FormatStream
	VideoFormats []FormatStream
	Progressive  []FormatStream
}

// StreamSelection holds the chosen audio/video streams, active bitrates, and quality options.
type StreamSelection struct {
	AudioURL           string          `json:"audio_url"`
	AudioUserAgent     string          `json:"-"`
	AudioBitrateKbps   int             `json:"audio_kbps"`
	VideoURL           string          `json:"video_url,omitempty"`
	VideoUserAgent     string          `json:"-"`
	VideoBitrateKbps   int             `json:"video_kbps"`
	RequestedQuality   int             `json:"requested_quality"`
	ActiveQuality      int             `json:"active_quality"`
	ProgressiveOnly    bool            `json:"progressive_only,omitempty"`
	AvailableQualities []QualityOption `json:"available_qualities,omitempty"`
}

type cachedStream struct {
	URL       string
	UserAgent string
	ExpiresAt time.Time
}

type cachedCatalog struct {
	Catalog   TrackCatalog
	ExpiresAt time.Time
}

type inflightResolve struct {
	done   chan struct{}
	stream ResolvedStream
	err    error
}

type inflightCatalogResolve struct {
	done    chan struct{}
	catalog TrackCatalog
	err     error
}

// Resolver resolves playable audio/video stream URLs and bitrates for YouTube video IDs.
type Resolver struct {
	httpClient *http.Client

	mu              sync.Mutex
	cache           map[string]cachedStream
	catalogs        map[string]cachedCatalog
	inflight        map[string]*inflightResolve
	inflightCatalog map[string]*inflightCatalogResolve
	ytdlp           string
	checked         bool
	auth            config.AuthState
}

// NewResolver creates a stream resolver with in-memory caching.
func NewResolver(httpClient *http.Client) *Resolver {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Resolver{
		httpClient:      httpClient,
		cache:           make(map[string]cachedStream),
		catalogs:        make(map[string]cachedCatalog),
		inflight:        make(map[string]*inflightResolve),
		inflightCatalog: make(map[string]*inflightCatalogResolve),
	}
}

// SetAuth updates auth state (cookies.txt is read when LoggedIn is true).
func (r *Resolver) SetAuth(auth config.AuthState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auth = auth
}

// Invalidate removes cached streams for videoID (e.g. after upstream 403/410).
func (r *Resolver) Invalidate(videoID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, videoID)
	delete(r.catalogs, videoID)
}

// Prefetch warms the stream catalog cache in the background for the upcoming track.
func (r *Resolver) Prefetch(videoID string) {
	if videoID == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, _ = r.ResolveCatalog(ctx, videoID)
	}()
}

// Resolve returns a direct CDN URL for the given YouTube video ID.
func (r *Resolver) Resolve(ctx context.Context, videoID string) (string, error) {
	st, err := r.ResolveStream(ctx, videoID)
	return st.URL, err
}

// ResolveStream returns a direct CDN URL and associated User-Agent for the given YouTube video ID.
func (r *Resolver) ResolveStream(ctx context.Context, videoID string) (ResolvedStream, error) {
	return r.ResolveStreamVariant(ctx, videoID, "", 0)
}

// ResolveStreamVariant resolves a specific stream variant ("audio", "video", or default/progressive)
// honoring the preferred video height when kind == "video".
func (r *Resolver) ResolveStreamVariant(ctx context.Context, videoID, kind string, prefHeight int) (ResolvedStream, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return ResolvedStream{}, errors.New("video ID is empty")
	}

	r.mu.Lock()
	if entry, ok := r.cache[videoID]; ok && time.Now().Before(entry.ExpiresAt) {
		r.mu.Unlock()
		return ResolvedStream{URL: entry.URL, UserAgent: entry.UserAgent}, nil
	}
	if kind == "" && prefHeight == 0 {
		if call, ok := r.inflight[videoID]; ok {
			r.mu.Unlock()
			select {
			case <-call.done:
				return call.stream, call.err
			case <-ctx.Done():
				return ResolvedStream{}, ctx.Err()
			}
		}
	}
	r.mu.Unlock()

	cat, err := r.ResolveCatalog(ctx, videoID)
	if err != nil {
		return ResolvedStream{}, err
	}

	if prefHeight <= 0 {
		prefHeight = defaultQuality
	}

	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "video":
		sel := SelectStreams(cat, true, prefHeight)
		if sel.VideoURL != "" {
			return ResolvedStream{URL: sel.VideoURL, UserAgent: sel.VideoUserAgent}, nil
		}
		if sel.AudioURL != "" {
			return ResolvedStream{URL: sel.AudioURL, UserAgent: sel.AudioUserAgent}, nil
		}
	case "audio":
		sel := SelectStreams(cat, false, prefHeight)
		if sel.AudioURL != "" {
			return ResolvedStream{URL: sel.AudioURL, UserAgent: sel.AudioUserAgent}, nil
		}
	default:
		if bestProg, ok := pickBestProgressive(cat.Progressive); ok {
			return ResolvedStream{URL: bestProg.URL, UserAgent: bestProg.UserAgent}, nil
		}
		sel := SelectStreams(cat, false, prefHeight)
		if sel.AudioURL != "" {
			return ResolvedStream{URL: sel.AudioURL, UserAgent: sel.AudioUserAgent}, nil
		}
	}

	return ResolvedStream{}, fmt.Errorf("no %s stream available for %s", kind, videoID)
}

// ResolveCatalogSelection resolves and selects audio/video streams for videoID according to preferences.
func (r *Resolver) ResolveCatalogSelection(ctx context.Context, videoID string, videoEnabled bool, prefHeight int) (StreamSelection, error) {
	cat, err := r.ResolveCatalog(ctx, videoID)
	if err != nil {
		return StreamSelection{}, err
	}
	sel := SelectStreams(cat, videoEnabled, prefHeight)
	if sel.AudioURL == "" && sel.VideoURL == "" {
		return StreamSelection{}, fmt.Errorf("unable to resolve playable stream for %s", videoID)
	}
	return sel, nil
}

// ResolveCatalog returns cached or freshly resolved TrackCatalog formats using single-flight deduplication.
func (r *Resolver) ResolveCatalog(ctx context.Context, videoID string) (TrackCatalog, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return TrackCatalog{}, errors.New("video ID is empty")
	}

	r.mu.Lock()
	if entry, ok := r.catalogs[videoID]; ok && time.Now().Before(entry.ExpiresAt) {
		r.mu.Unlock()
		return entry.Catalog, nil
	}
	call, ok := r.inflightCatalog[videoID]
	if !ok {
		call = &inflightCatalogResolve{done: make(chan struct{})}
		r.inflightCatalog[videoID] = call
		go func() {
			resolveCtx, resolveCancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer resolveCancel()

			cat, err := r.resolveCatalogUncached(resolveCtx, videoID)

			r.mu.Lock()
			if err == nil && (len(cat.AudioFormats) > 0 || len(cat.VideoFormats) > 0 || len(cat.Progressive) > 0) {
				r.catalogs[videoID] = cachedCatalog{
					Catalog:   cat,
					ExpiresAt: time.Now().Add(streamCacheTTL),
				}
			}
			delete(r.inflightCatalog, videoID)
			call.catalog = cat
			call.err = err
			close(call.done)
			r.mu.Unlock()
		}()
	}
	r.mu.Unlock()

	select {
	case <-call.done:
		return call.catalog, call.err
	case <-ctx.Done():
		return TrackCatalog{}, ctx.Err()
	}
}

// SelectStreams picks optimal audio & video streams for prefHeight, falling back to the next lower tier when unavailable.
func SelectStreams(cat TrackCatalog, videoEnabled bool, prefHeight int) StreamSelection {
	if prefHeight <= 0 {
		prefHeight = defaultQuality
	}

	sel := StreamSelection{
		RequestedQuality: prefHeight,
	}

	bestByHeight := make(map[int]FormatStream)
	updateTier := func(f FormatStream) {
		if f.URL == "" || f.Height <= 0 {
			return
		}
		cur, exists := bestByHeight[f.Height]
		if !exists || betterVideoCandidate(f, cur) {
			bestByHeight[f.Height] = f
		}
	}

	for _, vf := range cat.VideoFormats {
		updateTier(vf)
	}
	if len(bestByHeight) == 0 {
		for _, pf := range cat.Progressive {
			updateTier(pf)
		}
	}

	heights := make([]int, 0, len(bestByHeight))
	for h := range bestByHeight {
		heights = append(heights, h)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(heights)))

	if len(heights) > 0 {
		sel.AvailableQualities = make([]QualityOption, 0, len(heights))
		for _, h := range heights {
			vf := bestByHeight[h]
			lbl := fmt.Sprintf("%dp", h)
			sel.AvailableQualities = append(sel.AvailableQualities, QualityOption{
				Height: h,
				Label:  lbl,
				Kbps:   vf.BitrateKbps,
			})
		}
	}

	audio, hasAudio := pickBestAudio(cat.AudioFormats)
	prog, hasProg := pickBestProgressive(cat.Progressive)

	if hasAudio {
		sel.AudioURL = audio.URL
		sel.AudioUserAgent = audio.UserAgent
		sel.AudioBitrateKbps = audio.BitrateKbps
	} else if hasProg {
		sel.AudioURL = prog.URL
		sel.AudioUserAgent = prog.UserAgent
		sel.AudioBitrateKbps = prog.BitrateKbps
		sel.ProgressiveOnly = true
		if sel.ActiveQuality == 0 && prog.Height > 0 {
			sel.ActiveQuality = prog.Height
		}
	}

	if videoEnabled {
		chosenHeight := pickResolutionHeight(heights, prefHeight)
		if chosenHeight > 0 {
			chosenVid := bestByHeight[chosenHeight]
			sel.ActiveQuality = chosenHeight
			if chosenVid.Kind == "progressive" || !hasAudio {
				p := chosenVid
				if chosenVid.Kind != "progressive" || p.URL == "" {
					p = prog
				}
				if p.URL != "" {
					sel.AudioURL = p.URL
					sel.AudioUserAgent = p.UserAgent
					sel.AudioBitrateKbps = p.BitrateKbps
					if p.Height > 0 {
						sel.ActiveQuality = p.Height
					}
					sel.VideoURL = ""
					sel.VideoBitrateKbps = 0
					sel.ProgressiveOnly = true
				} else if chosenVid.URL != "" {
					sel.VideoURL = chosenVid.URL
					sel.VideoUserAgent = chosenVid.UserAgent
					sel.VideoBitrateKbps = chosenVid.BitrateKbps
				}
			} else {
				sel.VideoURL = chosenVid.URL
				sel.VideoUserAgent = chosenVid.UserAgent
				sel.VideoBitrateKbps = chosenVid.BitrateKbps
				sel.ProgressiveOnly = false
			}
		}
	}

	return sel
}

func pickResolutionHeight(descHeights []int, pref int) int {
	if len(descHeights) == 0 {
		return 0
	}
	for _, h := range descHeights {
		if h <= pref {
			return h
		}
	}
	// All available tiers exceed pref: fall back to lowest available tier.
	return descHeights[len(descHeights)-1]
}

func betterVideoCandidate(cand, cur FormatStream) bool {
	if cand.Kind != "progressive" && cur.Kind == "progressive" {
		return true
	}
	if cand.IsH264 != cur.IsH264 {
		return cand.IsH264
	}
	return cand.BitrateKbps > cur.BitrateKbps
}

func pickBestAudio(list []FormatStream) (FormatStream, bool) {
	if len(list) == 0 {
		return FormatStream{}, false
	}
	best := list[0]
	for _, f := range list[1:] {
		if f.URL == "" {
			continue
		}
		if best.URL == "" {
			best = f
			continue
		}
		if f.IsAAC != best.IsAAC {
			if f.IsAAC {
				best = f
			}
			continue
		}
		if f.BitrateKbps > best.BitrateKbps {
			best = f
		}
	}
	return best, best.URL != ""
}

func pickBestProgressive(list []FormatStream) (FormatStream, bool) {
	if len(list) == 0 {
		return FormatStream{}, false
	}
	var best FormatStream
	for _, f := range list {
		if f.URL == "" {
			continue
		}
		if best.URL == "" {
			best = f
			continue
		}
		if f.Itag == 18 && best.Itag != 18 {
			best = f
			continue
		}
		if best.Itag != 18 && f.IsH264 && !best.IsH264 {
			best = f
		}
	}
	return best, best.URL != ""
}

func (r *Resolver) resolveCatalogUncached(ctx context.Context, videoID string) (TrackCatalog, error) {
	if cat, err := r.resolveCatalogWithInnerTube(ctx, videoID); err == nil {
		return cat, nil
	}
	if bin := r.getYtDlpPath(); bin != "" {
		if cat, err := r.resolveCatalogWithYtDlp(ctx, bin, videoID); err == nil {
			return cat, nil
		}
		if st, err := r.resolveWithYtDlp(ctx, bin, videoID); err == nil && st.URL != "" {
			return TrackCatalog{
				Progressive: []FormatStream{{
					Itag:         18,
					URL:          st.URL,
					UserAgent:    st.UserAgent,
					Kind:         "progressive",
					MimeType:     "video/mp4",
					Height:       360,
					QualityLabel: "360p",
					IsH264:       true,
					IsAAC:        true,
				}},
			}, nil
		}
	}
	return TrackCatalog{}, fmt.Errorf("unable to resolve playable stream for %s", videoID)
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

func (r *Resolver) resolveCatalogWithYtDlp(ctx context.Context, bin, videoID string) (TrackCatalog, error) {
	args := []string{
		"-J",
		"--extractor-args", "youtube:player_client=android,web",
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
		return TrackCatalog{}, fmt.Errorf("yt-dlp -J failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	var meta struct {
		Formats []struct {
			FormatID    string            `json:"format_id"`
			URL         string            `json:"url"`
			Ext         string            `json:"ext"`
			Protocol    string            `json:"protocol"`
			Vcodec      string            `json:"vcodec"`
			Acodec      string            `json:"acodec"`
			Height      int               `json:"height"`
			FormatNote  string            `json:"format_note"`
			ABR         float64           `json:"abr"`
			VBR         float64           `json:"vbr"`
			TBR         float64           `json:"tbr"`
			HTTPHeaders map[string]string `json:"http_headers"`
		} `json:"formats"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &meta); err != nil {
		return TrackCatalog{}, err
	}

	var cat TrackCatalog
	for _, f := range meta.Formats {
		if !strings.HasPrefix(f.URL, "http://") && !strings.HasPrefix(f.URL, "https://") {
			continue
		}
		lowerProto := strings.ToLower(f.Protocol)
		lowerURL := strings.ToLower(f.URL)
		if strings.Contains(lowerProto, "m3u8") || strings.Contains(lowerURL, ".m3u8") || strings.Contains(lowerURL, "manifest/hls") ||
			strings.Contains(lowerProto, "dash") || strings.Contains(lowerURL, ".mpd") || strings.Contains(lowerURL, "manifest/dash") {
			continue
		}
		ua := f.HTTPHeaders["User-Agent"]
		if ua == "" {
			if strings.Contains(f.URL, "c=ANDROID") {
				ua = androidUserAgent
			} else if strings.Contains(f.URL, "c=IOS") {
				ua = iosUserAgent
			} else {
				ua = desktopChromeUA
			}
		}
		itag, _ := strconv.Atoi(f.FormatID)
		hasVideo := f.Vcodec != "" && f.Vcodec != "none"
		hasAudio := f.Acodec != "" && f.Acodec != "none"
		kbps := int(f.TBR + 0.5)
		if !hasVideo && hasAudio && f.ABR > 0 {
			kbps = int(f.ABR + 0.5)
		} else if hasVideo && !hasAudio && f.VBR > 0 {
			kbps = int(f.VBR + 0.5)
		}

		fs := FormatStream{
			Itag:         itag,
			URL:          f.URL,
			UserAgent:    ua,
			MimeType:     f.Ext,
			Height:       f.Height,
			QualityLabel: f.FormatNote,
			BitrateKbps:  kbps,
			IsH264:       strings.Contains(strings.ToLower(f.Vcodec), "avc1") || strings.Contains(strings.ToLower(f.Vcodec), "h264"),
			IsAAC:        strings.Contains(strings.ToLower(f.Acodec), "mp4a") || f.Ext == "m4a" || f.Ext == "mp4",
		}
		switch {
		case hasVideo && hasAudio:
			fs.Kind = "progressive"
			cat.Progressive = append(cat.Progressive, fs)
		case hasAudio && !hasVideo:
			fs.Kind = "audio"
			cat.AudioFormats = append(cat.AudioFormats, fs)
		case hasVideo && !hasAudio:
			if fs.Height == 0 {
				fs.Height = parseHeightFromLabel(fs.QualityLabel)
			}
			fs.Kind = "video"
			cat.VideoFormats = append(cat.VideoFormats, fs)
		}
	}

	filtered, ok := r.filterPlayableCatalog(ctx, cat)
	if !ok {
		return TrackCatalog{}, errors.New("yt-dlp -J returned no playable formats")
	}
	return filtered, nil
}

func (r *Resolver) resolveWithYtDlp(ctx context.Context, bin, videoID string) (ResolvedStream, error) {
	args := []string{
		"-g",
		"-f", "18/best[ext=mp4][acodec!=none][vcodec!=none][protocol!=m3u8_native][protocol!=m3u8]/best[acodec!=none][protocol!=m3u8_native][protocol!=m3u8]",
		"--extractor-args", "youtube:player_client=android,web",
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
		if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, ".m3u8") || strings.Contains(lower, "manifest/hls") {
			continue
		}
		ua := desktopChromeUA
		if strings.Contains(line, "c=ANDROID") {
			ua = androidUserAgent
		} else if strings.Contains(line, "c=IOS") {
			ua = iosUserAgent
		}
		return ResolvedStream{URL: line, UserAgent: ua}, nil
	}
	return ResolvedStream{}, errors.New("yt-dlp returned no progressive stream URL")
}

func (r *Resolver) resolveCatalogWithInnerTube(ctx context.Context, videoID string) (TrackCatalog, error) {
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
			userAgent: iosUserAgent,
		},
	}

	r.mu.Lock()
	loggedIn := r.auth.LoggedIn
	r.mu.Unlock()

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

		type rawFormat struct {
			Itag           int    `json:"itag"`
			URL            string `json:"url"`
			MimeType       string `json:"mimeType"`
			Bitrate        int    `json:"bitrate"`
			AverageBitrate int    `json:"averageBitrate"`
			Height         int    `json:"height"`
			QualityLabel   string `json:"qualityLabel"`
		}
		var data struct {
			PlayabilityStatus struct {
				Status string `json:"status"`
				Reason string `json:"reason"`
			} `json:"playabilityStatus"`
			StreamingData struct {
				Formats         []rawFormat `json:"formats"`
				AdaptiveFormats []rawFormat `json:"adaptiveFormats"`
			} `json:"streamingData"`
		}
		if err := json.Unmarshal(respBytes, &data); err != nil {
			lastErr = err
			continue
		}

		status := strings.ToUpper(strings.TrimSpace(data.PlayabilityStatus.Status))
		if status != "" && status != "OK" {
			lastErr = fmt.Errorf("playability %s: %s", status, strings.TrimSpace(data.PlayabilityStatus.Reason))
			if status == "LOGIN_REQUIRED" && loggedIn {
				break
			}
			continue
		}

		var cat TrackCatalog
		parseKbps := func(avg, peak int) int {
			bps := avg
			if bps <= 0 {
				bps = peak
			}
			if bps <= 0 {
				return 0
			}
			kbps := (bps + 500) / 1000
			if kbps < 1 {
				return 1
			}
			return kbps
		}

		for _, f := range data.StreamingData.Formats {
			if f.URL == "" {
				continue
			}
			h := f.Height
			if h <= 0 {
				h = parseHeightFromLabel(f.QualityLabel)
			}
			if h <= 0 && f.Itag == 18 {
				h = 360
			}
			cat.Progressive = append(cat.Progressive, FormatStream{
				Itag:         f.Itag,
				URL:          f.URL,
				UserAgent:    cl.userAgent,
				Kind:         "progressive",
				MimeType:     f.MimeType,
				Height:       h,
				QualityLabel: f.QualityLabel,
				BitrateKbps:  parseKbps(f.AverageBitrate, f.Bitrate),
				IsH264:       strings.Contains(f.MimeType, "avc1") || strings.Contains(f.MimeType, "video/mp4"),
				IsAAC:        strings.Contains(f.MimeType, "mp4a") || strings.Contains(f.MimeType, "video/mp4"),
			})
		}

		for _, f := range data.StreamingData.AdaptiveFormats {
			if f.URL == "" {
				continue
			}
			lowerMime := strings.ToLower(f.MimeType)
			kbps := parseKbps(f.AverageBitrate, f.Bitrate)
			switch {
			case strings.HasPrefix(lowerMime, "audio/"):
				isAAC := strings.Contains(lowerMime, "mp4a")
				// Skip non-AAC MP4 surround codecs (ec-3/ac-3) that fail decoding in Chromium/WebView2.
				if strings.HasPrefix(lowerMime, "audio/mp4") && !isAAC {
					continue
				}
				if !strings.HasPrefix(lowerMime, "audio/mp4") && !strings.HasPrefix(lowerMime, "audio/webm") {
					continue
				}
				cat.AudioFormats = append(cat.AudioFormats, FormatStream{
					Itag:        f.Itag,
					URL:         f.URL,
					UserAgent:   cl.userAgent,
					Kind:        "audio",
					MimeType:    f.MimeType,
					BitrateKbps: kbps,
					IsAAC:       isAAC,
				})
			case strings.HasPrefix(lowerMime, "video/"):
				h := f.Height
				if h <= 0 {
					h = parseHeightFromLabel(f.QualityLabel)
				}
				if h <= 0 {
					continue
				}
				cat.VideoFormats = append(cat.VideoFormats, FormatStream{
					Itag:         f.Itag,
					URL:          f.URL,
					UserAgent:    cl.userAgent,
					Kind:         "video",
					MimeType:     f.MimeType,
					Height:       h,
					QualityLabel: f.QualityLabel,
					BitrateKbps:  kbps,
					IsH264:       strings.Contains(lowerMime, "avc1"),
				})
			}
		}

		if filtered, ok := r.filterPlayableCatalog(ctx, cat); ok {
			return filtered, nil
		}
		lastErr = errors.New("no playable format URL passed probe")
	}
	if lastErr == nil {
		lastErr = errors.New("failed to resolve stream URL")
	}
	return TrackCatalog{}, lastErr
}

// filterPlayableCatalog verifies adaptive and progressive streams accept open-ended Range requests
// (as sent by Chromium/WebView2) and strips gated adaptive streams so playback falls back cleanly.
func (r *Resolver) filterPlayableCatalog(ctx context.Context, cat TrackCatalog) (TrackCatalog, bool) {
	if len(cat.AudioFormats) > 0 {
		bestAudio, hasAudio := pickBestAudio(cat.AudioFormats)
		if hasAudio && r.probeStreamURL(ctx, bestAudio.URL, bestAudio.UserAgent) {
			if len(cat.VideoFormats) > 0 {
				if repVid, hasVid := pickRepresentativeVideo(cat.VideoFormats); !hasVid || !r.probeStreamURL(ctx, repVid.URL, repVid.UserAgent) {
					cat.VideoFormats = nil
				}
			}
			return cat, true
		}
		// Adaptive audio is gated (e.g. HTTP 403 without PO token); drop adaptive streams and check progressive.
		cat.AudioFormats = nil
		cat.VideoFormats = nil
	}

	if bestProg, hasProg := pickBestProgressive(cat.Progressive); hasProg && r.probeStreamURL(ctx, bestProg.URL, bestProg.UserAgent) {
		return cat, true
	}
	return TrackCatalog{}, false
}

func pickRepresentativeVideo(list []FormatStream) (FormatStream, bool) {
	if len(list) == 0 {
		return FormatStream{}, false
	}
	repSel := SelectStreams(TrackCatalog{VideoFormats: list}, true, defaultQuality)
	for _, f := range list {
		if repSel.VideoURL != "" && f.URL == repSel.VideoURL {
			return f, true
		}
	}
	for _, f := range list {
		if f.URL != "" {
			return f, true
		}
	}
	return FormatStream{}, false
}

func parseHeightFromLabel(label string) int {
	label = strings.TrimSpace(strings.ToLower(label))
	idx := strings.IndexByte(label, 'p')
	if idx <= 0 {
		return 0
	}
	h, err := strconv.Atoi(label[:idx])
	if err != nil || h <= 0 {
		return 0
	}
	return h
}

func (r *Resolver) probeStreamURL(ctx context.Context, streamURL, ua string) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, streamURL, nil)
	if err != nil {
		return false
	}
	// Open-ended range matches Chromium/WebView2 media requests and exposes PO-token 403 gates immediately.
	req.Header.Set("Range", "bytes=0-")
	req.Header.Set("User-Agent", ua)
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent
}
