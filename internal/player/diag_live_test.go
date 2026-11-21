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
	id := "e-ORhEE9VVg"
	r := NewResolver(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cat, err := r.resolveCatalogWithInnerTube(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	sel := SelectStreams(cat, true, 720)
	// Test video itag 136 beyond 2MB
	for _, rng := range []string{"bytes=1500000-2000000", "bytes=2097152-2621439", "bytes=5000000-5500000"} {
		testURL2(ctx, r.httpClient, "IT-V-136", sel.VideoURL, sel.VideoUserAgent, rng)
	}
	// Test FULL progressive itag 18 download size / tail
	if len(cat.Progressive) > 0 {
		testURL2(ctx, r.httpClient, "IT-P-18-tail", cat.Progressive[0].URL, cat.Progressive[0].UserAgent, "bytes=8000000-8165468")
		testURL2(ctx, r.httpClient, "IT-P-18-open", cat.Progressive[0].URL, cat.Progressive[0].UserAgent, "bytes=0-")
	}
	// Test IOS audio itag 249 / 251 / 139 / 140
	for _, af := range cat.AudioFormats {
		testURL2(ctx, r.httpClient, fmt.Sprintf("IT-A-%d-1.5M", af.Itag), af.URL, af.UserAgent, "bytes=1500000-1565535")
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
