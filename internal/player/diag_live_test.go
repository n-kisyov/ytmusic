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
	id := "IdneKLhsWOQ"
	r := NewResolver(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Check IdneKLhsWOQ video 136/137 tail and audio 250/251 open
	cat, err := r.resolveCatalogWithInnerTube(ctx, id)
	if err == nil {
		sel := SelectStreams(cat, true, 720)
		testURL2(ctx, r.httpClient, "Idn-V-136-21M", sel.VideoURL, sel.VideoUserAgent, "bytes=21000000-21781357")
		testURL2(ctx, r.httpClient, "Idn-V-136-10M", sel.VideoURL, sel.VideoUserAgent, "bytes=10000000-10100000")
		for _, af := range cat.AudioFormats {
			testURL2(ctx, r.httpClient, fmt.Sprintf("Idn-A-%d-open", af.Itag), af.URL, af.UserAgent, "bytes=0-")
		}
	}

	// Check IOS versions for hlsManifestUrl + WEB with signatureTimestamp
	cls := []struct {
		name string
		ctx  map[string]any
		ua   string
		body map[string]any
	}{
		{"WEB_STS", map[string]any{"clientName": "WEB", "clientVersion": "2.20250220.00.00", "hl": "en", "gl": "US"}, desktopChromeUA, map[string]any{"playbackContext": map[string]any{"contentPlaybackContext": map[string]any{"signatureTimestamp": 20130}}}},
		{"MWEB_STS", map[string]any{"clientName": "MWEB", "clientVersion": "2.20250220.00.00", "hl": "en", "gl": "US"}, "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", map[string]any{"playbackContext": map[string]any{"contentPlaybackContext": map[string]any{"signatureTimestamp": 20130}}}},
		{"IOS_1929", map[string]any{"clientName": "IOS", "clientVersion": "19.29.1", "deviceMake": "Apple", "deviceModel": "iPhone16,2", "osName": "iPhone", "osVersion": "17.5.1.21F90", "hl": "en", "gl": "US"}, "com.google.ios.youtube/19.29.1 (iPhone16,2; U; CPU iOS 17_5_1 like Mac OS X)", nil},
	}
	for _, cl := range cls {
		p := map[string]any{"context": map[string]any{"client": cl.ctx}, "videoId": id, "contentCheckOk": true, "racyCheckOk": true}
		for k, v := range cl.body {
			p[k] = v
		}
		b, _ := json.Marshal(p)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.youtube.com/youtubei/v1/player?prettyPrint=false", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", cl.ua)
		resp, err := r.httpClient.Do(req)
		if err != nil {
			continue
		}
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		var d struct {
			PlayabilityStatus struct{ Status, Reason string } `json:"playabilityStatus"`
			StreamingData     struct {
				HlsManifestUrl  string `json:"hlsManifestUrl"`
				Formats         []struct{ Itag int; URL string } `json:"formats"`
				AdaptiveFormats []struct{ Itag int; URL string } `json:"adaptiveFormats"`
			} `json:"streamingData"`
		}
		_ = json.Unmarshal(rb, &d)
		fmt.Printf("cl=%s http=%d play=%s (%s) prog=%d adap=%d hls=%v\n",
			cl.name, resp.StatusCode, d.PlayabilityStatus.Status, d.PlayabilityStatus.Reason,
			len(d.StreamingData.Formats), len(d.StreamingData.AdaptiveFormats), d.StreamingData.HlsManifestUrl != "")
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
	fmt.Printf("[%s] status=%d cl=%q cr=%q read=%d\n", tag, resp.StatusCode, resp.Header.Get("Content-Length"), resp.Header.Get("Content-Range"), n)
}
