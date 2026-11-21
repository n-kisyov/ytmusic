package youtube

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ytmusic/internal/config"
)

const (
	defaultUserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	webClientName     = "WEB"
	webClientVersion  = "2.20250115.01.00"
	youtubeOrigin     = "https://www.youtube.com"
	defaultSuggestURL = "https://suggestqueries.google.com/complete/search"
)

// Client interacts with YouTube InnerTube APIs for search and playlist continuation.
type Client struct {
	httpClient *http.Client
	baseURL    string
	suggestURL string

	mu       sync.Mutex
	auth     config.AuthState
	overflow []config.Track
}

// NewClient creates a new YouTube InnerTube client.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    youtubeOrigin,
		suggestURL: defaultSuggestURL,
	}
}

// SetBaseURL overrides the YouTube base URL (useful for testing with httptest).
func (c *Client) SetBaseURL(baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = strings.TrimRight(baseURL, "/")
}

// SetSuggestURL overrides the YouTube autocomplete endpoint (useful for testing with httptest).
func (c *Client) SetSuggestURL(suggestURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.suggestURL = suggestURL
}

// SetAuth updates the authentication state used for InnerTube requests.
func (c *Client) SetAuth(auth config.AuthState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.auth = auth
}

// ClearOverflow clears any buffered overflow tracks from a previous search.
func (c *Client) ClearOverflow() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.overflow = nil
}

// SetOverflow restores buffered overflow tracks (e.g. from persisted state).
func (c *Client) SetOverflow(tracks []config.Track) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(tracks) == 0 {
		c.overflow = nil
		return
	}
	c.overflow = append([]config.Track(nil), tracks...)
}

// GetOverflow returns a copy of currently buffered overflow tracks.
func (c *Client) GetOverflow() []config.Track {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.overflow) == 0 {
		return nil
	}
	return append([]config.Track(nil), c.overflow...)
}

// SearchInitialPlaylist searches YouTube and collects targetCount playable tracks (default 30),
// automatically following continuation tokens if a single search page returns fewer tracks.
func (c *Client) SearchInitialPlaylist(ctx context.Context, query string, targetCount int) (SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return SearchResult{}, errors.New("search query cannot be empty")
	}
	if targetCount <= 0 {
		targetCount = 30
	}

	c.ClearOverflow()

	seen := make(map[string]struct{}, targetCount+16)
	tracks := make([]config.Track, 0, targetCount+16)

	// Request video results ("EgIQAQ==") first; if empty, retry unfiltered search.
	page, err := c.postSearch(ctx, map[string]any{
		"context": c.buildContext(),
		"query":   query,
		"params":  "EgIQAQ==",
	})
	if err != nil {
		return SearchResult{}, err
	}

	pageTracks, token := extractTracksAndToken(page)
	if len(pageTracks) == 0 {
		page, err = c.postSearch(ctx, map[string]any{
			"context": c.buildContext(),
			"query":   query,
		})
		if err != nil {
			return SearchResult{}, err
		}
		pageTracks, token = extractTracksAndToken(page)
	}

	tracks = appendUniqueTracks(tracks, pageTracks, seen)

	// Automatically fetch continuations until targetCount (30) tracks are gathered.
	for attempts := 0; len(tracks) < targetCount && token != "" && attempts < 4; attempts++ {
		nextPage, err := c.postSearch(ctx, map[string]any{
			"context":      c.buildContext(),
			"continuation": token,
		})
		if err != nil {
			break
		}
		moreTracks, nextToken := extractTracksAndToken(nextPage)
		prevLen := len(tracks)
		tracks = appendUniqueTracks(tracks, moreTracks, seen)
		token = nextToken
		if len(tracks) == prevLen && token == "" {
			break
		}
	}

	// If YouTube search still yielded fewer than targetCount, supplement with Radio Mix (/next) from the first track.
	if len(tracks) < targetCount && len(tracks) > 0 {
		mixTracks, mixToken, err := c.fetchNextMix(ctx, tracks[0].ID, "")
		if err == nil && len(mixTracks) > 0 {
			tracks = appendUniqueTracks(tracks, mixTracks, seen)
			if token == "" && mixToken != "" {
				token = prefixContinuation("next", mixToken)
			}
		}
	}

	if len(tracks) > targetCount {
		c.mu.Lock()
		c.overflow = append([]config.Track(nil), tracks[targetCount:]...)
		c.mu.Unlock()
		tracks = tracks[:targetCount]
	}

	return SearchResult{
		Tracks:            tracks,
		ContinuationToken: token,
	}, nil
}

// FetchMoreSongs retrieves additional songs using buffered overflow, continuation token,
// or YouTube Mix (/youtubei/v1/next) seeded from seedVideoID.
func (c *Client) FetchMoreSongs(ctx context.Context, continuationToken, seedVideoID string, existingIDs map[string]struct{}) (SearchResult, error) {
	seen := make(map[string]struct{}, len(existingIDs)+32)
	for id := range existingIDs {
		seen[id] = struct{}{}
	}

	var collected []config.Track

	// Drain in-memory overflow from initial 30-track trim first.
	c.mu.Lock()
	if len(c.overflow) > 0 {
		collected = appendUniqueTracks(collected, c.overflow, seen)
		c.overflow = nil
	}
	c.mu.Unlock()

	if len(collected) >= 10 {
		return SearchResult{
			Tracks:            collected,
			ContinuationToken: continuationToken,
		}, nil
	}

	token := continuationToken
	if token != "" {
		source, endpoint, rawTok := parseContinuation(token)
		page, err := c.postInnerTube(ctx, endpoint, map[string]any{
			"context":      c.buildContext(),
			"continuation": rawTok,
		})
		if err != nil {
			if len(collected) > 0 {
				return SearchResult{
					Tracks:            collected,
					ContinuationToken: token,
				}, nil
			}
			return SearchResult{}, err
		}
		moreTracks, nextToken := extractTracksAndToken(page)
		collected = appendUniqueTracks(collected, moreTracks, seen)
		token = prefixContinuation(source, nextToken)
	}

	// Fallback to /youtubei/v1/next Radio Mix only when continuation is exhausted.
	if (len(collected) == 0 || (len(collected) < 5 && token == "")) && seedVideoID != "" {
		mixTracks, mixToken, err := c.fetchNextMix(ctx, seedVideoID, "")
		if err != nil {
			if len(collected) == 0 {
				return SearchResult{}, err
			}
		} else {
			collected = appendUniqueTracks(collected, mixTracks, seen)
			if token == "" && mixToken != "" {
				token = prefixContinuation("next", mixToken)
			}
		}
	}

	return SearchResult{
		Tracks:            collected,
		ContinuationToken: token,
	}, nil
}

// ParseYouTubeInput detects YouTube video/playlist links and extracts videoID and playlistID.
func ParseYouTubeInput(raw string) (videoID, playlistID string, isURL bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", false
	}
	lower := strings.ToLower(trimmed)
	if !strings.Contains(lower, "youtube.com") && !strings.Contains(lower, "youtu.be") {
		return "", "", false
	}
	candidate := trimmed
	if !strings.Contains(lower, "://") {
		candidate = "https://" + trimmed
	}
	u, err := url.Parse(candidate)
	if err != nil {
		return "", "", false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "youtu.be":
		seg := strings.Trim(u.Path, "/")
		if seg != "" {
			videoID = strings.Split(seg, "/")[0]
		}
		playlistID = strings.TrimSpace(u.Query().Get("list"))
		return videoID, playlistID, videoID != "" || playlistID != ""
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		q := u.Query()
		videoID = strings.TrimSpace(q.Get("v"))
		playlistID = strings.TrimSpace(q.Get("list"))
		if videoID == "" {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 2 {
				switch parts[0] {
				case "shorts", "embed", "live":
					videoID = parts[1]
				}
			}
		}
		return videoID, playlistID, videoID != "" || playlistID != ""
	default:
		return "", "", false
	}
}

// ResolveURLPlaylist resolves a pasted YouTube watch/playlist URL into a playable queue.
func (c *Client) ResolveURLPlaylist(ctx context.Context, rawURL string, targetCount int) (SearchResult, string, error) {
	videoID, playlistID, ok := ParseYouTubeInput(rawURL)
	if !ok {
		return SearchResult{}, "", errors.New("unsupported YouTube URL")
	}
	if targetCount <= 0 {
		targetCount = 30
	}
	c.ClearOverflow()

	if playlistID != "" {
		res, err := c.fetchBrowsePlaylist(ctx, playlistID, videoID, targetCount)
		if err == nil && len(res.Tracks) > 0 {
			return res, "Loaded YouTube playlist", nil
		}
		if videoID != "" {
			mixTracks, mixToken, errNext := c.fetchNextMixList(ctx, videoID, playlistID)
			if errNext == nil && len(mixTracks) > 0 {
				return c.trimAndStoreOverflow(mixTracks, prefixContinuation("next", mixToken), targetCount), "Loaded YouTube playlist", nil
			}
		}
	}

	if videoID != "" {
		res, err := c.StartRadioMix(ctx, videoID, nil, targetCount)
		if err != nil {
			return SearchResult{}, "", err
		}
		return res, "Loaded YouTube track & Radio Mix", nil
	}
	return SearchResult{}, "", errors.New("no playable tracks found at YouTube URL")
}

// StartRadioMix builds a 30-song Radio Mix queue seeded from seedVideoID.
func (c *Client) StartRadioMix(ctx context.Context, seedVideoID string, seedTrack *config.Track, targetCount int) (SearchResult, error) {
	seedVideoID = strings.TrimSpace(seedVideoID)
	if seedVideoID == "" {
		return SearchResult{}, errors.New("seed video ID cannot be empty")
	}
	if targetCount <= 0 {
		targetCount = 30
	}
	c.ClearOverflow()

	seen := make(map[string]struct{}, targetCount+16)
	tracks := make([]config.Track, 0, targetCount+16)

	if seedTrack != nil && seedTrack.ID != "" {
		tracks = appendUniqueTracks(tracks, []config.Track{*seedTrack}, seen)
	} else if det, err := c.fetchVideoDetailsTrack(ctx, seedVideoID); err == nil && det.ID != "" {
		tracks = appendUniqueTracks(tracks, []config.Track{det}, seen)
	}

	mixTracks, token, err := c.fetchNextMix(ctx, seedVideoID, "")
	if err != nil && len(tracks) == 0 {
		return SearchResult{}, err
	}
	tracks = appendUniqueTracks(tracks, mixTracks, seen)

	for attempts := 0; len(tracks) < targetCount && token != "" && attempts < 2; attempts++ {
		more, nextTok, nextErr := c.fetchNextMix(ctx, seedVideoID, token)
		if nextErr != nil {
			break
		}
		prevLen := len(tracks)
		tracks = appendUniqueTracks(tracks, more, seen)
		token = nextTok
		if len(tracks) == prevLen && token == "" {
			break
		}
	}

	return c.trimAndStoreOverflow(tracks, prefixContinuation("next", token), targetCount), nil
}

func (c *Client) trimAndStoreOverflow(tracks []config.Track, token string, targetCount int) SearchResult {
	if len(tracks) > targetCount {
		c.mu.Lock()
		c.overflow = append([]config.Track(nil), tracks[targetCount:]...)
		c.mu.Unlock()
		tracks = tracks[:targetCount]
	}
	return SearchResult{
		Tracks:            tracks,
		ContinuationToken: token,
	}
}

func (c *Client) fetchBrowsePlaylist(ctx context.Context, playlistID, startVideoID string, targetCount int) (SearchResult, error) {
	browseID := playlistID
	if !strings.HasPrefix(browseID, "VL") {
		browseID = "VL" + browseID
	}
	resp, err := c.postInnerTube(ctx, "/youtubei/v1/browse?prettyPrint=false", map[string]any{
		"context":  c.buildContext(),
		"browseId": browseID,
	})
	if err != nil {
		return SearchResult{}, err
	}
	seen := make(map[string]struct{}, targetCount+16)
	first, token := extractTracksAndToken(resp)
	tracks := appendUniqueTracks(nil, first, seen)

	for attempts := 0; len(tracks) < targetCount && token != "" && attempts < 3; attempts++ {
		nextResp, nextErr := c.postInnerTube(ctx, "/youtubei/v1/browse?prettyPrint=false", map[string]any{
			"context":      c.buildContext(),
			"continuation": token,
		})
		if nextErr != nil {
			break
		}
		more, nextTok := extractTracksAndToken(nextResp)
		prevLen := len(tracks)
		tracks = appendUniqueTracks(tracks, more, seen)
		token = nextTok
		if len(tracks) == prevLen && token == "" {
			break
		}
	}

	if startVideoID != "" {
		for i, tr := range tracks {
			if tr.ID == startVideoID && i > 0 {
				reordered := make([]config.Track, 0, len(tracks))
				reordered = append(reordered, tracks[i:]...)
				reordered = append(reordered, tracks[:i]...)
				tracks = reordered
				break
			}
		}
	}

	return c.trimAndStoreOverflow(tracks, prefixContinuation("browse", token), targetCount), nil
}

func (c *Client) fetchVideoDetailsTrack(ctx context.Context, videoID string) (config.Track, error) {
	resp, err := c.postInnerTube(ctx, "/youtubei/v1/player?prettyPrint=false", map[string]any{
		"context":        c.buildContext(),
		"videoId":        videoID,
		"contentCheckOk": true,
		"racyCheckOk":    true,
	})
	if err != nil {
		return config.Track{}, err
	}
	vd, ok := resp["videoDetails"].(map[string]any)
	if !ok {
		return config.Track{}, errors.New("missing videoDetails")
	}
	title, _ := vd["title"].(string)
	author, _ := vd["author"].(string)
	lenStr, _ := vd["lengthSeconds"].(string)
	durSec, _ := strconv.Atoi(strings.TrimSpace(lenStr))
	if title == "" || durSec <= 0 {
		return config.Track{}, errors.New("invalid videoDetails")
	}
	if author == "" {
		author = "YouTube"
	}
	thumb := extractThumbnailURL(vd["thumbnail"], videoID)
	return config.Track{
		ID:           videoID,
		Title:        title,
		Artist:       author,
		DurationSec:  durSec,
		DurationText: formatDurationClock(durSec),
		Thumbnail:    thumb,
	}, nil
}

func formatDurationClock(sec int) string {
	if sec <= 0 {
		return "0:00"
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// SuggestQueries retrieves live YouTube search autocomplete suggestions for partial query text.
func (c *Client) SuggestQueries(ctx context.Context, query string) ([]string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []string{}, nil
	}
	if _, _, isURL := ParseYouTubeInput(query); isURL {
		return []string{}, nil
	}

	c.mu.Lock()
	endpoint := c.suggestURL
	c.mu.Unlock()
	if endpoint == "" {
		endpoint = defaultSuggestURL
	}

	reqURL := endpoint + "?client=firefox&ds=yt&q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("suggest HTTP %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return nil, err
	}

	var parsed []any
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed) < 2 {
		return []string{}, nil
	}
	arr, ok := parsed[1].([]any)
	if !ok {
		return []string{}, nil
	}
	out := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" || strings.EqualFold(s, query) {
			continue
		}
		low := strings.ToLower(s)
		if _, dup := seen[low]; dup {
			continue
		}
		seen[low] = struct{}{}
		out = append(out, s)
		if len(out) >= 8 {
			break
		}
	}
	return out, nil
}

func prefixContinuation(source, tok string) string {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return ""
	}
	if strings.HasPrefix(tok, "browse:") || strings.HasPrefix(tok, "next:") {
		return tok
	}
	switch source {
	case "browse", "next":
		return source + ":" + tok
	default:
		return tok
	}
}

func parseContinuation(raw string) (source, endpoint, tok string) {
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "browse:"):
		return "browse", "/youtubei/v1/browse?prettyPrint=false", strings.TrimPrefix(raw, "browse:")
	case strings.HasPrefix(raw, "next:"):
		return "next", "/youtubei/v1/next?prettyPrint=false", strings.TrimPrefix(raw, "next:")
	default:
		return "search", "/youtubei/v1/search?prettyPrint=false", strings.TrimPrefix(raw, "search:")
	}
}

func (c *Client) fetchNextMix(ctx context.Context, videoID, continuation string) ([]config.Track, string, error) {
	return c.fetchNextMixList(ctx, videoID, "RD"+videoID, continuation)
}

func (c *Client) fetchNextMixList(ctx context.Context, videoID, playlistID string, continuationOpt ...string) ([]config.Track, string, error) {
	continuation := ""
	if len(continuationOpt) > 0 {
		continuation = continuationOpt[0]
	}
	payload := map[string]any{
		"context": c.buildContext(),
	}
	if continuation != "" {
		_, _, rawTok := parseContinuation(continuation)
		payload["continuation"] = rawTok
	} else {
		payload["videoId"] = videoID
		if playlistID != "" {
			payload["playlistId"] = playlistID
		}
	}
	resp, err := c.postInnerTube(ctx, "/youtubei/v1/next?prettyPrint=false", payload)
	if err != nil {
		return nil, "", err
	}
	tracks, token := extractTracksAndToken(resp)
	return tracks, token, nil
}

func (c *Client) postSearch(ctx context.Context, payload map[string]any) (map[string]any, error) {
	return c.postInnerTube(ctx, "/youtubei/v1/search?prettyPrint=false", payload)
}

func (c *Client) postInnerTube(ctx context.Context, endpoint string, payload map[string]any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal innertube payload: %w", err)
	}

	c.mu.Lock()
	base := c.baseURL
	auth := c.auth
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create innertube request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("X-YouTube-Client-Name", "1")
	req.Header.Set("X-YouTube-Client-Version", webClientVersion)
	req.Header.Set("Origin", youtubeOrigin)
	req.Header.Set("Referer", youtubeOrigin+"/")

	if auth.LoggedIn && len(auth.Cookies) > 0 {
		applyAuthHeaders(req, auth, time.Now().Unix())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("innertube request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read innertube response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("innertube returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}

	var parsed map[string]any
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("decode innertube JSON: %w", err)
	}
	return parsed, nil
}

func (c *Client) buildContext() map[string]any {
	c.mu.Lock()
	activePageID := c.auth.ActivePageID
	loggedIn := c.auth.LoggedIn
	c.mu.Unlock()

	ctx := map[string]any{
		"client": map[string]any{
			"clientName":       webClientName,
			"clientVersion":    webClientVersion,
			"hl":               "en",
			"gl":               "US",
			"utcOffsetMinutes": 0,
		},
	}
	if loggedIn && activePageID != "" {
		ctx["user"] = map[string]any{
			"onBehalfOfUser": activePageID,
		}
	}
	return ctx
}

func applyAuthHeaders(req *http.Request, auth config.AuthState, unixTS int64) {
	cookiePairs := make([]string, 0, len(auth.Cookies))
	for k, v := range auth.Cookies {
		if k != "" && v != "" {
			cookiePairs = append(cookiePairs, k+"="+v)
		}
	}
	if len(cookiePairs) > 0 {
		req.Header.Set("Cookie", strings.Join(cookiePairs, "; "))
	}

	sapisid := auth.Cookies["SAPISID"]
	if sapisid == "" {
		sapisid = auth.Cookies["__Secure-3PAPISID"]
	}
	if sapisid != "" {
		req.Header.Set("Authorization", ComputeSAPISIDHash(sapisid, youtubeOrigin, unixTS))
		req.Header.Set("X-Origin", youtubeOrigin)
	}
	if auth.ActivePageID != "" {
		req.Header.Set("X-Goog-PageId", auth.ActivePageID)
	}
	authUser := auth.AuthUser
	if authUser == "" {
		authUser = "0"
	}
	req.Header.Set("X-Goog-AuthUser", authUser)
}

// ComputeSAPISIDHash calculates the YouTube SAPISIDHASH Authorization header value.
func ComputeSAPISIDHash(sapisid, origin string, unixTS int64) string {
	raw := fmt.Sprintf("%d %s %s", unixTS, sapisid, origin)
	sum := sha1.Sum([]byte(raw))
	return fmt.Sprintf("SAPISIDHASH %d_%s", unixTS, hex.EncodeToString(sum[:]))
}

func appendUniqueTracks(dst, src []config.Track, seen map[string]struct{}) []config.Track {
	for _, t := range src {
		if t.ID == "" {
			continue
		}
		if _, exists := seen[t.ID]; exists {
			continue
		}
		seen[t.ID] = struct{}{}
		dst = append(dst, t)
	}
	return dst
}

// extractTracksAndToken walks arbitrary InnerTube JSON responses and extracts playable tracks
// and the continuation token (preferring continuationItemRenderer over generic commands).
func extractTracksAndToken(root any) ([]config.Track, string) {
	var tracks []config.Track
	var feedContinuation string
	var fallbackContinuation string

	var findToken func(node any) string
	findToken = func(node any) string {
		switch v := node.(type) {
		case map[string]any:
			if cc, ok := v["continuationCommand"].(map[string]any); ok {
				if tok, ok := cc["token"].(string); ok && tok != "" {
					return tok
				}
			}
			for _, child := range v {
				if tok := findToken(child); tok != "" {
					return tok
				}
			}
		case []any:
			for _, item := range v {
				if tok := findToken(item); tok != "" {
					return tok
				}
			}
		}
		return ""
	}

	// Primary keys are visited first in deterministic order; secondary recommendation rails are deferred.
	primaryKeyOrder := []string{
		"onResponseReceivedCommands",
		"onResponseReceivedActions",
		"twoColumnBrowseResultsRenderer",
		"twoColumnWatchNextResults",
		"singleColumnWatchNextResults",
		"playlist",
		"playlistPanelRenderer",
		"primaryContents",
		"sectionListRenderer",
		"itemSectionRenderer",
		"playlistVideoListRenderer",
		"contents",
	}
	isSecondaryKey := func(k string) bool {
		return k == "secondaryResults" || k == "secondaryContents"
	}

	var walk func(node any, allowSecondary bool)
	walk = func(node any, allowSecondary bool) {
		switch v := node.(type) {
		case map[string]any:
			if vr, ok := v["videoRenderer"].(map[string]any); ok {
				if t, ok := parseVideoRenderer(vr); ok {
					tracks = append(tracks, t)
				}
				return
			}
			if pvr, ok := v["playlistVideoRenderer"].(map[string]any); ok {
				if t, ok := parseVideoRenderer(pvr); ok {
					tracks = append(tracks, t)
				}
				return
			}
			if pr, ok := v["playlistPanelVideoRenderer"].(map[string]any); ok {
				if t, ok := parseVideoRenderer(pr); ok {
					tracks = append(tracks, t)
				}
				return
			}
			if cr, ok := v["compactVideoRenderer"].(map[string]any); ok {
				if t, ok := parseVideoRenderer(cr); ok {
					tracks = append(tracks, t)
				}
				return
			}
			if lvm, ok := v["lockupViewModel"].(map[string]any); ok {
				if t, ok := parseLockupViewModel(lvm); ok {
					tracks = append(tracks, t)
				}
				return
			}
			if cir, ok := v["continuationItemRenderer"].(map[string]any); ok {
				if tok := findToken(cir); tok != "" && feedContinuation == "" {
					feedContinuation = tok
				}
				return
			}
			if cc, ok := v["continuationCommand"].(map[string]any); ok {
				if tok, ok := cc["token"].(string); ok && tok != "" && fallbackContinuation == "" {
					fallbackContinuation = tok
				}
			}

			visited := make(map[string]struct{}, len(v))
			for _, k := range primaryKeyOrder {
				if child, exists := v[k]; exists {
					visited[k] = struct{}{}
					walk(child, allowSecondary)
				}
			}
			rest := make([]string, 0, len(v))
			for k := range v {
				if _, seen := visited[k]; !seen && !isSecondaryKey(k) {
					rest = append(rest, k)
				}
			}
			sort.Strings(rest)
			for _, k := range rest {
				walk(v[k], allowSecondary)
			}
			if allowSecondary {
				for _, k := range []string{"secondaryResults", "secondaryContents"} {
					if child, exists := v[k]; exists && len(tracks) == 0 {
						walk(child, true)
					}
				}
			}
		case []any:
			for _, item := range v {
				walk(item, allowSecondary)
			}
		}
	}

	walk(root, false)
	if len(tracks) == 0 {
		walk(root, true)
	}
	if feedContinuation != "" {
		return tracks, feedContinuation
	}
	return tracks, fallbackContinuation
}

func parseVideoRenderer(m map[string]any) (config.Track, bool) {
	id, _ := m["videoId"].(string)
	if id == "" {
		return config.Track{}, false
	}

	title := extractText(m["title"])
	if title == "" {
		return config.Track{}, false
	}

	durationText := extractText(m["lengthText"])
	durationSec := ParseDurationSeconds(durationText)
	if durationSec <= 0 {
		if ls, ok := m["lengthSeconds"].(string); ok && ls != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(ls)); err == nil && parsed > 0 {
				durationSec = parsed
				durationText = formatDurationClock(parsed)
			}
		}
	}
	if durationSec <= 0 {
		// Skip live broadcasts, shorts shelves without duration, or upcoming premieres.
		return config.Track{}, false
	}

	artist := extractText(m["ownerText"])
	if artist == "" {
		artist = extractText(m["longBylineText"])
	}
	if artist == "" {
		artist = extractText(m["shortBylineText"])
	}
	if artist == "" {
		artist = "YouTube"
	}

	thumb := extractThumbnailURL(m["thumbnail"], id)

	return config.Track{
		ID:           id,
		Title:        title,
		Artist:       artist,
		DurationSec:  durationSec,
		DurationText: durationText,
		Thumbnail:    thumb,
	}, true
}

func parseLockupViewModel(m map[string]any) (config.Track, bool) {
	contentType, _ := m["contentType"].(string)
	if contentType != "" && !strings.Contains(contentType, "VIDEO") {
		return config.Track{}, false
	}
	id, _ := m["contentId"].(string)
	if id == "" {
		return config.Track{}, false
	}

	var title, artist, durationText string
	if meta, ok := m["metadata"].(map[string]any); ok {
		if lockupMeta, ok := meta["lockupMetadataViewModel"].(map[string]any); ok {
			if titleObj, ok := lockupMeta["title"].(map[string]any); ok {
				title, _ = titleObj["content"].(string)
			}
			if metaRows, ok := lockupMeta["metadata"].(map[string]any); ok {
				if contentMeta, ok := metaRows["contentMetadataViewModel"].(map[string]any); ok {
					if rows, ok := contentMeta["metadataRows"].([]any); ok && len(rows) > 0 {
						if row0, ok := rows[0].(map[string]any); ok {
							if parts, ok := row0["metadataParts"].([]any); ok && len(parts) > 0 {
								if part0, ok := parts[0].(map[string]any); ok {
									if textObj, ok := part0["text"].(map[string]any); ok {
										artist, _ = textObj["content"].(string)
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Find duration text inside thumbnail overlay badges.
	var findBadge func(node any)
	findBadge = func(node any) {
		if durationText != "" {
			return
		}
		switch v := node.(type) {
		case map[string]any:
			if text, ok := v["text"].(string); ok && ParseDurationSeconds(text) > 0 {
				durationText = text
				return
			}
			for _, child := range v {
				findBadge(child)
			}
		case []any:
			for _, item := range v {
				findBadge(item)
			}
		}
	}
	findBadge(m["contentImage"])

	durationSec := ParseDurationSeconds(durationText)
	if title == "" || durationSec <= 0 {
		return config.Track{}, false
	}
	if artist == "" {
		artist = "YouTube"
	}

	return config.Track{
		ID:           id,
		Title:        title,
		Artist:       artist,
		DurationSec:  durationSec,
		DurationText: durationText,
		Thumbnail:    fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", id),
	}, true
}

func extractText(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	if s, ok := m["simpleText"].(string); ok && s != "" {
		return strings.TrimSpace(s)
	}
	if runs, ok := m["runs"].([]any); ok && len(runs) > 0 {
		var b strings.Builder
		for _, r := range runs {
			if rm, ok := r.(map[string]any); ok {
				if text, ok := rm["text"].(string); ok {
					b.WriteString(text)
				}
			}
		}
		return strings.TrimSpace(b.String())
	}
	return ""
}

func extractBestThumbnailURL(v any) string {
	if m, ok := v.(map[string]any); ok {
		if list, ok := m["thumbnails"].([]any); ok && len(list) > 0 {
			for i := len(list) - 1; i >= 0; i-- {
				if item, ok := list[i].(map[string]any); ok {
					if url, ok := item["url"].(string); ok && url != "" {
						if strings.HasPrefix(url, "//") {
							return "https:" + url
						}
						return url
					}
				}
			}
		}
	}
	return ""
}

func extractThumbnailURL(v any, videoID string) string {
	if u := extractBestThumbnailURL(v); u != "" {
		return u
	}
	return fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", videoID)
}

// ParseDurationSeconds parses durations like "3:45" or "1:02:03" into seconds.
// Returns 0 if the input is not a valid positive media duration.
func ParseDurationSeconds(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	parts := strings.Split(text, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	total := 0
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			return 0
		}
		total = total*60 + n
	}
	return total
}
