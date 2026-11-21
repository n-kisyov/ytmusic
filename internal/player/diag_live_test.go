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
	for _, rng := range []string{
		"bytes=0-262143",
		"bytes=0-524287",
		"bytes=0-1048575",
		"bytes=0-4410720",
		"bytes=524288-1048575",
		"bytes=4000000-4410720",
	} {
		testURL2(ctx, r.httpClient, "IT-A", sel.AudioURL, sel.AudioUserAgent, rng)
	}
	for _, rng := range []string{
		"bytes=0-524287",
		"bytes=0-1048575",
		"bytes=0-2097151",
		"bytes=524288-1048575",
		"bytes=22000000-22404755",
	} {
		testURL2(ctx, r.httpClient, "IT-V", sel.VideoURL, sel.VideoUserAgent, rng)
	}
}

func testURL2(ctx context.Context, hc *http.Client, tag, u, ua, rng string) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Range", rng)
	resp, err := hc.Do(req)
	if err != nil {
		fmt.Printf("[%s] rng=%q ERR=%v\n", tag, rng, err)
		return
	}
	n, _ := io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	fmt.Printf("[%s] rng=%q status=%d ct=%q cl=%q cr=%q read=%d\n",
		tag, rng, resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Length"), resp.Header.Get("Content-Range"), n)
}
