package player

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func TestResolveStreamSharedInflightSurvivesFirstCallerCancel(t *testing.T) {
	resolver := NewResolver(nil)
	callStarted := make(chan struct{})
	releaseProbe := make(chan struct{})

	// Inject an in-flight resolution simulating a slow network call whose first caller cancels early.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-callStarted:
		default:
			close(callStarted)
		}
		<-releaseProbe
		w.WriteHeader(http.StatusPartialContent)
	}))
	defer srv.Close()

	// Test ResolveStream's detached context directly via inflight waiter sharing:
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()

	var wg sync.WaitGroup
	var err1, err2 error
	var st2 ResolvedStream

	// Prime a fake inflight call backed by probeStreamURL latency using two ResolveStream calls:
	// first create a custom resolver whose resolveUncached calls srv, or reuse inflight directly.
	call := &inflightResolve{done: make(chan struct{})}
	resolver.mu.Lock()
	resolver.inflight["vid_inflight"] = call
	resolver.mu.Unlock()

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err1 = resolver.ResolveStream(ctx1, "vid_inflight")
	}()
	go func() {
		defer wg.Done()
		st2, err2 = resolver.ResolveStream(ctx2, "vid_inflight")
	}()

	time.Sleep(25 * time.Millisecond)
	cancel1()

	resolver.mu.Lock()
	call.stream = ResolvedStream{URL: srv.URL + "/stream.mp4", UserAgent: "test-ua"}
	delete(resolver.inflight, "vid_inflight")
	close(call.done)
	resolver.mu.Unlock()

	wg.Wait()
	if !errors.Is(err1, context.Canceled) {
		t.Fatalf("expected caller 1 context.Canceled, got %v", err1)
	}
	if err2 != nil || st2.URL != srv.URL+"/stream.mp4" {
		t.Fatalf("expected caller 2 to succeed with resolved URL, got err=%v stream=%+v", err2, st2)
	}

	// Verify detached probe context doesn't inherit a canceled parent context.
	if !resolver.probeStreamURL(context.Background(), func() string {
		close(releaseProbe)
		return srv.URL
	}(), "test-ua") {
		t.Fatalf("expected probeStreamURL to succeed")
	}
}

func TestProxyRangeForwardingAndUIStaticServing(t *testing.T) {
	payload := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var gotUA string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
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
		UserAgent: "custom-test-ua/1.0",
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

	// Verify Range request forwarding and User-Agent preservation.
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
	if gotUA != "custom-test-ua/1.0" {
		t.Fatalf("expected upstream User-Agent 'custom-test-ua/1.0', got %q", gotUA)
	}
}

func TestLiveStreamResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live stream resolve in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	resolver := NewResolver(nil)
	st, err := resolver.ResolveStream(ctx, "2QzQUO3j0gE")
	if err != nil {
		t.Fatalf("ResolveStream failed: %v", err)
	}
	if !strings.HasPrefix(st.URL, "http") || st.UserAgent == "" {
		t.Fatalf("expected http stream URL and non-empty UserAgent, got %+v", st)
	}
	t.Logf("resolved stream URL length=%d ua=%q", len(st.URL), st.UserAgent)

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

func TestSelectStreamsDefault720AndFallbackAnd1080Preference(t *testing.T) {
	cat := TrackCatalog{
		AudioFormats: []FormatStream{
			{Itag: 139, URL: "http://a/139", Kind: "audio", BitrateKbps: 49, IsAAC: true},
			{Itag: 140, URL: "http://a/140", Kind: "audio", BitrateKbps: 130, IsAAC: true},
			{Itag: 251, URL: "http://a/251", Kind: "audio", BitrateKbps: 161, IsAAC: false},
		},
		VideoFormats: []FormatStream{
			{Itag: 137, URL: "http://v/1080-h264", Kind: "video", Height: 1080, QualityLabel: "1080p", BitrateKbps: 2135, IsH264: true},
			{Itag: 248, URL: "http://v/1080-vp9", Kind: "video", Height: 1080, QualityLabel: "1080p", BitrateKbps: 1072, IsH264: false},
			{Itag: 135, URL: "http://v/480-h264", Kind: "video", Height: 480, QualityLabel: "480p", BitrateKbps: 494, IsH264: true},
			{Itag: 134, URL: "http://v/360-h264", Kind: "video", Height: 360, QualityLabel: "360p", BitrateKbps: 295, IsH264: true},
		},
		Progressive: []FormatStream{
			{Itag: 18, URL: "http://p/18", Kind: "progressive", Height: 360, BitrateKbps: 424, IsH264: true, IsAAC: true},
		},
	}

	// Audio-only mode picks AAC itag 140 (130 kbps) and leaves video empty.
	audOnly := SelectStreams(cat, false, 720)
	if audOnly.AudioURL != "http://a/140" || audOnly.AudioBitrateKbps != 130 || audOnly.VideoURL != "" || audOnly.VideoBitrateKbps != 0 {
		t.Fatalf("unexpected audio-only selection: %+v", audOnly)
	}

	// Video enabled with default 720p when 720p is unavailable steps down to next lower (480p).
	defVid := SelectStreams(cat, true, 0)
	if defVid.RequestedQuality != 720 || defVid.ActiveQuality != 480 || defVid.VideoURL != "http://v/480-h264" || defVid.VideoBitrateKbps != 494 {
		t.Fatalf("expected 720p pref without 720p tier to pick 480p, got %+v", defVid)
	}

	// Video enabled with 1080p picks H.264 1080p over VP9 1080p.
	hdVid := SelectStreams(cat, true, 1080)
	if hdVid.ActiveQuality != 1080 || hdVid.VideoURL != "http://v/1080-h264" || hdVid.VideoBitrateKbps != 2135 {
		t.Fatalf("expected 1080p H.264 selection, got %+v", hdVid)
	}
	if len(hdVid.AvailableQualities) != 3 || hdVid.AvailableQualities[0].Height != 1080 {
		t.Fatalf("expected 3 unique qualities sorted descending, got %+v", hdVid.AvailableQualities)
	}
}
