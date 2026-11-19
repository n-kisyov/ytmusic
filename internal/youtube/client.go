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
	"strconv"
	"strings"
	"sync"
	"time"

	"ytmusic/internal/config"
)

const (
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	webClientName    = "WEB"
	webClientVersion = "2.20250115.01.00"
	youtubeOrigin    = "https://www.youtube.com"
)

// Client interacts with YouTube InnerTube APIs for search and playlist continuation.
type Client struct {
	httpClient *http.Client
	baseURL    string

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
	}
}

// SetBaseURL overrides the YouTube base URL (useful for testing with httptest).
func (c *Client) SetBaseURL(baseURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = strings.TrimRight(baseURL, "/")
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
			if token == "" {
				token = mixToken
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
		page, err := c.postSearch(ctx, map[string]any{
			"context":      c.buildContext(),
			"continuation": token,
		})
		if err == nil {
			moreTracks, nextToken := extractTracksAndToken(page)
			collected = appendUniqueTracks(collected, moreTracks, seen)
			token = nextToken
		} else {
			token = ""
		}
	}

	// Fallback to /youtubei/v1/next Radio Mix if continuation yielded nothing or was exhausted.
	if len(collected) == 0 && seedVideoID != "" {
		mixTracks, _, err := c.fetchNextMix(ctx, seedVideoID, "")
		if err != nil {
			return SearchResult{}, err
		}
		collected = appendUniqueTracks(collected, mixTracks, seen)
	}

	return SearchResult{
		Tracks:            collected,
		ContinuationToken: token,
	}, nil
}

func (c *Client) fetchNextMix(ctx context.Context, videoID, continuation string) ([]config.Track, string, error) {
	payload := map[string]any{
		"context": c.buildContext(),
	}
	if continuation != "" {
		payload["continuation"] = continuation
	} else {
		payload["videoId"] = videoID
		payload["playlistId"] = "RD" + videoID
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
// and the continuation token.
func extractTracksAndToken(root any) ([]config.Track, string) {
	var tracks []config.Track
	var continuation string

	var walk func(node any)
	walk = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			if vr, ok := v["videoRenderer"].(map[string]any); ok {
				if t, ok := parseVideoRenderer(vr); ok {
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
			if cc, ok := v["continuationCommand"].(map[string]any); ok {
				if tok, ok := cc["token"].(string); ok && tok != "" {
					continuation = tok
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, item := range v {
				walk(item)
			}
		}
	}

	walk(root)
	return tracks, continuation
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
