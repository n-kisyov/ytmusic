package player

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestDiagLiveStreams(t *testing.T) {
	r := NewResolver(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// 1. Check whether 2QzQUO3j0gE (non-Vevo) also 403s on adaptive bytes=0- or tail
	if cat, err := r.resolveCatalogWithInnerTube(ctx, "2QzQUO3j0gE"); err == nil {
		sel := SelectStreams(cat, true, 720)
		fmt.Printf("2QzQUO3j0gE audio=%dkbps video=%dkbps(%dp)\n", sel.AudioBitrateKbps, sel.VideoBitrateKbps, sel.ActiveQuality)
		testURL2(ctx, r.httpClient, "2Qz-A-open", sel.AudioURL, sel.AudioUserAgent, "bytes=0-")
		testURL2(ctx, r.httpClient, "2Qz-V-open", sel.VideoURL, sel.VideoUserAgent, "bytes=0-")
	}

	// 2. Test other InnerTube clients on e-ORhEE9VVg
	alts := []struct {
		name string
		ctx  map[string]any
		ua   string
	}{
		{"ANDROID_CREATOR", map[string]any{"clientName": "ANDROID_CREATOR", "clientVersion": "24.47.100", "androidSdkVersion": 34, "osName": "Android", "osVersion": "14", "hl": "en", "gl": "US"}, "com.google.android.apps.youtube.creator/24.47.100 (Linux; U; Android 14) gzip"},
		{"IOS_CREATOR", map[string]any{"clientName": "IOS_CREATOR", "clientVersion": "24.47.100", "deviceMake": "Apple", "deviceModel": "iPhone16,2", "osName": "iPhone", "osVersion": "18.1.0.22B83", "hl": "en", "gl": "US"}, iosUserAgent},
		{"IOS_MUSIC", map[string]any{"clientName": "IOS_MUSIC", "clientVersion": "7.29", "deviceMake": "Apple", "deviceModel": "iPhone16,2", "osName": "iPhone", "osVersion": "18.1.0.22B83", "hl": "en", "gl": "US"}, "com.google.ios.youtubemusic/7.29 (iPhone16,2; U; CPU iOS 18_1_0 like Mac OS X;)"},
		{"MEDIA_CONNECT", map[string]any{"clientName": "MEDIA_CONNECT_FRONTEND", "clientVersion": "0.1", "hl": "en", "gl": "US"}, desktopChromeUA},
		{"ANDROID_LITE", map[string]any{"clientName": "ANDROID_LITE", "clientVersion": "3.26.1", "androidSdkVersion": 30, "osName": "Android", "osVersion": "11", "hl": "en", "gl": "US"}, "com.google.android.youtube.lite/3.26.1 (Linux; U; Android 11) gzip"},
		{"IOS_EMBEDDED", map[string]any{"clientName": "IOS_EMBEDDED_PLAYER", "clientVersion": "1.20241118.01.00", "hl": "en", "gl": "US"}, iosUserAgent},
	}

	for _, cl := range alts {
		payload := map[string]any{
			"context":        map[string]any{"client": cl.ctx},
			"videoId":        "e-ORhEE9VVg",
			"contentCheckOk": true,
			"racyCheckOk":    true,
		}
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.youtube.com/youtubei/v1/player?prettyPrint=false", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", cl.ua)
		resp, err := r.httpClient.Do(req)
		if err != nil {
			fmt.Printf("alt=%s ERR=%v\n", cl.name, err)
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		var d struct {
			PlayabilityStatus struct {
				Status string `json:"status"`
				Reason string `json:"reason"`
			} `json:"playabilityStatus"`
			StreamingData struct {
				HlsManifestUrl  string `json:"hlsManifestUrl"`
				Formats         []struct{ Itag int; URL string } `json:"formats"`
				AdaptiveFormats []struct{ Itag int; URL string; Height int } `json:"adaptiveFormats"`
			} `json:"streamingData"`
		}
		_ = json.Unmarshal(b, &d)
		fmt.Printf("alt=%s status=%d play=%s (%s) prog=%d adap=%d hls=%v\n",
			cl.name, resp.StatusCode, d.PlayabilityStatus.Status, d.PlayabilityStatus.Reason,
			len(d.StreamingData.Formats), len(d.StreamingData.AdaptiveFormats), d.StreamingData.HlsManifestUrl != "")
		for _, f := range d.StreamingData.AdaptiveFormats {
			if (f.Itag == 140 || f.Itag == 136) && f.URL != "" {
				testURL2(ctx, r.httpClient, fmt.Sprintf("%s-%d", cl.name, f.Itag), f.URL, cl.ua, "bytes=0-")
			}
		}
	}
}

func testURL2(ctx context.Context, hc *http.Client, tag, u, ua, rng string) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("User-Agent", ua)
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	resp, err := hc.Do(req)
	if err != nil {
		fmt.Printf("[%s] ERR=%v\n", tag, err)
		return
	}
	n, _ := io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	fmt.Printf("[%s] rng=%q status=%d cl=%q cr=%q read=%d\n", tag, rng, resp.StatusCode, resp.Header.Get("Content-Length"), resp.Header.Get("Content-Range"), n)
}
