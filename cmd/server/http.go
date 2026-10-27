package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/geiserx/duplicacy-mcp/config"
	"github.com/mark3labs/mcp-go/server"
)

const (
	endpointPath = "/mcp"
	// maxRequestBytes bounds a request body. MCP requests to this server are
	// a few hundred bytes.
	maxRequestBytes = 1 << 20
)

// newHTTPHandler wraps the Streamable HTTP transport in the protections the
// SDK does not provide: Origin validation, bearer authentication and a body
// size limit. The SDK still applies its own DNS-rebinding Host check.
func newHTTPHandler(s *server.MCPServer, cfg config.HTTPConfig) (http.Handler, error) {
	var h http.Handler = server.NewStreamableHTTPServer(s,
		server.WithEndpointPath(endpointPath),
		server.WithStateLess(true),
	)
	h = limitBody(h, maxRequestBytes)
	if cfg.AuthToken != "" {
		h = requireBearer(h, cfg.AuthToken)
	}
	cop := http.NewCrossOriginProtection()
	for _, origin := range cfg.AllowedOrigins {
		if err := cop.AddTrustedOrigin(origin); err != nil {
			return nil, fmt.Errorf("MCP_ALLOWED_ORIGINS: %w", err)
		}
	}
	h = cop.Handler(h)

	mux := http.NewServeMux()
	mux.Handle(endpointPath, h)
	return mux, nil
}

// newHTTPServer returns an http.Server with timeouts, so slow or idle
// clients cannot hold connections open forever.
func newHTTPServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}
}

// limitBody answers 413 to a body declared larger than n, and caps the read
// of one with no declared length.
func limitBody(next http.Handler, n int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > n {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, n)
		next.ServeHTTP(w, r)
	})
}

// requireBearer accepts only "Authorization: Bearer <token>". The compare is
// over SHA-256 digests in constant time, so neither the content nor the
// length of the token leaks through timing.
func requireBearer(next http.Handler, token string) http.Handler {
	want := sha256.Sum256([]byte(token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := bearerToken(r.Header.Get("Authorization"))
		sum := sha256.Sum256([]byte(got))
		if !ok || subtle.ConstantTimeCompare(sum[:], want[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="duplicacy-mcp"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bearerToken extracts the token from an Authorization header. A header
// without the Bearer scheme yields false.
func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}
