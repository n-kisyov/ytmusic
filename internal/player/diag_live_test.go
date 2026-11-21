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
	ids := []string{"e-ORhEE9VVg", "nfWlot6h_JM", "IdneKLhsWOQ"}
	r := NewResolver(nil)
	srv, err := NewServer(r, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	for _, id := range ids {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		cat, err := r.ResolveCatalog(ctx, id)
		if err != nil {
			fmt.Printf("=== %s RESOLVE ERR: %v\n", id, err)
			cancel()
			continue
		}
		sel := SelectStreams(cat, true, 720)
		fmt.Printf("=== %s Audio=%d Video=%d Prog=%d UA=%q SelectedA=%dkbps(itag=%d) SelectedV=%dkbps(%dp, itag=%d)\n",
			id, len(cat.AudioFormats), len(cat.VideoFormats), len(cat.Progressive), cat.UserAgent,
			sel.AudioBitrateKbps, findItag(cat.AudioFormats, sel.AudioURL),
			sel.VideoBitrateKbps, sel.ActiveQuality, findItag(cat.VideoFormats, sel.VideoURL),
		)
		for _, kind := range []string{"audio", "video"} {
			q := 0
			if kind == "video" {
				q = 720
			}
			u := srv.StreamVariantURL(id, kind, q)
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
			req.Header.Set("Range", "bytes=0-1023")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Printf("  proxy %s ERR: %v\n", kind, err)
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("  proxy %s status=%d ct=%q cl=%q cr=%q bytes=%d\n",
				kind, resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Length"), resp.Header.Get("Content-Range"), len(body))
		}
		cancel()
	}
}

func findItag(fmts []FormatStream, u string) int {
	for _, f := range fmts {
		if f.URL == u {
			return f.Itag
		}
	}
	return -1
}
