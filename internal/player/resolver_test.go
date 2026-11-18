package player

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestProxyRangeForwardingAndUIStaticServing(t *testing.T) {
	payload := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Accept-Ranges", "bytes")
		if r.Header.Get("Range") == "bytes=0-9" {
			w.Header().Set("Content-Range", "bytes 0-9/36")
			w.Header().Set("Content-Length", "10")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(w, payload[:10])
			return
		}
		w.Header().Set("Content-Length", "36")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, payload)
	}))
	defer upstream.Close()

	resolver := NewResolver(nil)
	resolver.mu.Lock()
	resolver.cache["testvid"] = cachedStream{
		URL:       upstream.URL,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	resolver.mu.Unlock()

	memFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>ok</html>")},
	}
	srv, err := NewServer(resolver, memFS)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer srv.Close()

	// Verify static UI serving.
	uiResp, err := http.Get(srv.BaseURL() + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	uiBody, _ := io.ReadAll(uiResp.Body)
	uiResp.Body.Close()
	if !strings.Contains(string(uiBody), "<html>ok</html>") {
		t.Fatalf("unexpected UI response: %q", string(uiBody))
	}

	// Verify Range request forwarding.
	req, _ := http.NewRequest(http.MethodGet, srv.StreamURL("testvid"), nil)
	req.Header.Set("Range", "bytes=0-9")
	streamResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /stream failed: %v", err)
	}
	defer streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content, got %d", streamResp.StatusCode)
	}
	chunk, _ := io.ReadAll(streamResp.Body)
	if string(chunk) != "0123456789" {
		t.Fatalf("expected '0123456789', got %q", string(chunk))
	}
}

func TestLiveStreamResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live stream resolve in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resolver := NewResolver(nil)
	url, err := resolver.Resolve(ctx, "2QzQUO3j0gE")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if !strings.HasPrefix(url, "http") {
		t.Fatalf("expected http stream URL, got %q", url)
	}
	t.Logf("resolved stream URL length=%d", len(url))

	srv, err := NewServer(resolver, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer srv.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.StreamURL("2QzQUO3j0gE"), nil)
	req.Header.Set("Range", "bytes=0-1023")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("proxy stream GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("expected 206 Partial Content from proxy, got %d", resp.StatusCode)
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read proxy stream body failed: %v", err)
	}
	if len(buf) != 1024 {
		t.Fatalf("expected 1024 bytes from range request, got %d", len(buf))
	}
}
