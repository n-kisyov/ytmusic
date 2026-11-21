package player

import (
	"context"
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

	for _, id := range []string{"2QzQUO3j0gE", "IdneKLhsWOQ"} {
		cat, err := r.resolveCatalogWithInnerTube(ctx, id)
		if err != nil {
			fmt.Printf("%s err=%v\n", id, err)
			continue
		}
		sel := SelectStreams(cat, true, 720)
		fmt.Printf("=== %s prog=%d adapA=%d adapV=%d\n", id, len(cat.Progressive), len(cat.AudioFormats), len(cat.VideoFormats))
		testURL2(ctx, r.httpClient, id+"-A-0-100k", sel.AudioURL, sel.AudioUserAgent, "bytes=0-102399")
		testURL2(ctx, r.httpClient, id+"-A-2M", sel.AudioURL, sel.AudioUserAgent, "bytes=2000000-2100000")
		testURL2(ctx, r.httpClient, id+"-V-0-100k", sel.VideoURL, sel.VideoUserAgent, "bytes=0-102399")
		testURL2(ctx, r.httpClient, id+"-V-5M", sel.VideoURL, sel.VideoUserAgent, "bytes=5000000-5100000")
		if len(cat.Progressive) > 0 {
			testURL2(ctx, r.httpClient, id+"-P18-open", cat.Progressive[0].URL, cat.Progressive[0].UserAgent, "bytes=0-")
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
