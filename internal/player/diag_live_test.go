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
		af := findFmt(cat.AudioFormats, sel.AudioURL)
		vf := findFmt(cat.VideoFormats, sel.VideoURL)
		fmt.Printf("=== %s Audio=%d Video=%d Prog=%d\n  SelA: itag=%d %dkbps mime=%q ua=%q\n  SelV: itag=%d %dkbps %dp mime=%q ua=%q\n",
			id, len(cat.AudioFormats), len(cat.VideoFormats), len(cat.Progressive),
			af.Itag, sel.AudioBitrateKbps, af.MimeType, sel.AudioUserAgent,
			vf.Itag, sel.VideoBitrateKbps, sel.ActiveQuality, vf.MimeType, sel.VideoUserAgent,
		)
		for _, tc := range []struct {
			kind string
			q    int
		}{{"audio", 0}, {"video", 720}, {"", 0}} {
			u := srv.StreamVariantURL(id, tc.kind, tc.q)
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
			req.Header.Set("Range", "bytes=0-1023")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Printf("  proxy kind=%q q=%d ERR: %v\n", tc.kind, tc.q, err)
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Printf("  proxy kind=%q q=%d status=%d ct=%q cl=%q cr=%q bytes=%d\n",
				tc.kind, tc.q, resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Length"), resp.Header.Get("Content-Range"), len(body))
		}
		cancel()
	}
}

func findFmt(fmts []FormatStream, u string) FormatStream {
	for _, f := range fmts {
		if f.URL == u {
			return f
		}
	}
	return FormatStream{}
}
