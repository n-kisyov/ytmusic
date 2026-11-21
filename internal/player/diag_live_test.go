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

	for _, cl := range innerTubeClients {
		cat, err := r.fetchCatalogInnerTube(ctx, id, cl)
		if err != nil {
			fmt.Printf("client=%s err=%v\n", cl.name, err)
			continue
		}
		sel := SelectStreams(cat, true, 720)
		fmt.Printf("client=%s audio=%d video=%d prog=%d selA=%d selV=%d\n",
			cl.name, len(cat.AudioFormats), len(cat.VideoFormats), len(cat.Progressive),
			findItag2(cat.AudioFormats, sel.AudioURL), findItag2(cat.VideoFormats, sel.VideoURL))
		for _, rng := range []string{"bytes=0-1023", "bytes=0-65535", "bytes=0-", ""} {
			testURL(ctx, r.client, cl.name+"-A", sel.AudioURL, cl.userAgent, rng)
			testURL(ctx, r.client, cl.name+"-V", sel.VideoURL, cl.userAgent, rng)
		}
	}

	if bin := r.getYtDlpPath(); bin != "" {
		cat, err := r.resolveCatalogWithYtDlp(ctx, bin, id)
		if err != nil {
			fmt.Printf("ytdlp err=%v\n", err)
		} else {
			sel := SelectStreams(cat, true, 720)
			fmt.Printf("client=ytdlp audio=%d video=%d prog=%d selA=%d selV=%d ua=%q\n",
				len(cat.AudioFormats), len(cat.VideoFormats), len(cat.Progressive),
				findItag2(cat.AudioFormats, sel.AudioURL), findItag2(cat.VideoFormats, sel.VideoURL), sel.AudioUserAgent)
			for _, rng := range []string{"bytes=0-1023", "bytes=0-", ""} {
				testURL(ctx, r.client, "ytdlp-A", sel.AudioURL, sel.AudioUserAgent, rng)
				testURL(ctx, r.client, "ytdlp-V", sel.VideoURL, sel.VideoUserAgent, rng)
			}
		}
	}
}

func testURL(ctx context.Context, hc *http.Client, tag, u, ua, rng string) {
	if u == "" {
		return
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	resp, err := hc.Do(req)
	if err != nil {
		fmt.Printf("  [%s] rng=%q ERR=%v\n", tag, rng, err)
		return
	}
	n, _ := io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	fmt.Printf("  [%s] rng=%q status=%d ct=%q cr=%q read=%d\n", tag, rng, resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Range"), n)
}

func findItag2(fmts []FormatStream, u string) int {
	for _, f := range fmts {
		if f.URL == u {
			return f.Itag
		}
	}
	return -1
}
