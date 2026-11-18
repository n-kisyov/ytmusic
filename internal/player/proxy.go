package player

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"
)

// Server serves the embedded UI assets and proxies YouTube media streams with HTTP Range support.
type Server struct {
	resolver   *Resolver
	httpServer *http.Server
	proxyHTTP  *http.Client
	baseURL    string
}

// NewServer creates and starts a local HTTP server bound to 127.0.0.1:0.
func NewServer(resolver *Resolver, uiFS fs.FS) (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	s := &Server{
		resolver: resolver,
		proxyHTTP: &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   15 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ResponseHeaderTimeout: 20 * time.Second,
				MaxIdleConns:          32,
				IdleConnTimeout:       90 * time.Second,
			},
		},
		baseURL: "http://" + ln.Addr().String(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", s.handleStream)
	if uiFS != nil {
		fileServer := http.FileServer(http.FS(uiFS))
		mux.Handle("/", fileServer)
	}

	s.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			_ = err
		}
	}()

	return s, nil
}

// BaseURL returns the local server base URL (e.g. "http://127.0.0.1:51234").
func (s *Server) BaseURL() string {
	return s.baseURL
}

// StreamURL returns the local proxy URL for a given track ID.
func (s *Server) StreamURL(videoID string) string {
	return s.baseURL + "/stream?id=" + videoID
}

// Close shuts down the local HTTP server gracefully.
func (s *Server) Close() error {
	if s.httpServer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	videoID := strings.TrimSpace(r.URL.Query().Get("id"))
	if videoID == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	var resp *http.Response
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		streamURL, resolveErr := s.resolver.Resolve(r.Context(), videoID)
		if resolveErr != nil {
			http.Error(w, "failed to resolve stream: "+resolveErr.Error(), http.StatusBadGateway)
			return
		}

		req, reqErr := http.NewRequestWithContext(r.Context(), r.Method, streamURL, nil)
		if reqErr != nil {
			http.Error(w, "invalid upstream URL", http.StatusInternalServerError)
			return
		}
		if rng := r.Header.Get("Range"); rng != "" {
			req.Header.Set("Range", rng)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
		req.Header.Set("Referer", "https://www.youtube.com/")
		req.Header.Set("Origin", "https://www.youtube.com")

		resp, err = s.proxyHTTP.Do(req)
		if err != nil {
			if attempt == 0 {
				s.resolver.Invalidate(videoID)
				continue
			}
			http.Error(w, "upstream stream error: "+err.Error(), http.StatusBadGateway)
			return
		}

		if (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusGone) && attempt == 0 {
			resp.Body.Close()
			s.resolver.Invalidate(videoID)
			continue
		}
		break
	}
	if resp == nil {
		http.Error(w, "upstream response unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeader := func(name string) {
		if v := resp.Header.Get(name); v != "" {
			w.Header().Set(name, v)
		}
	}
	copyHeader("Content-Type")
	copyHeader("Content-Length")
	copyHeader("Content-Range")
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "video/mp4")
	}
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)

	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, resp.Body)
}
