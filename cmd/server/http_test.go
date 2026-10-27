package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/geiserx/duplicacy-mcp/config"
)

const testToken = "0123456789abcdef0123456789abcdef"

func startHTTP(t *testing.T, cfg config.HTTPConfig) *httptest.Server {
	t.Helper()
	h, err := newHTTPHandler(testServer(t), cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func modernToolsList() []byte {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list", "params": modern(nil)})
	return b
}

func post(t *testing.T, url string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url+endpointPath, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

var modernHeaders = map[string]string{"Mcp-Protocol-Version": "2026-07-28", "Mcp-Method": "tools/list"}

func with(extra map[string]string) map[string]string {
	h := map[string]string{}
	for k, v := range modernHeaders {
		h[k] = v
	}
	for k, v := range extra {
		h[k] = v
	}
	return h
}

// decode reads a JSON or single-event SSE response body.
func decode(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	b, _ := io.ReadAll(resp.Body)
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		sc := bufio.NewScanner(bytes.NewReader(b))
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				b = []byte(data)
				break
			}
		}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("body %q: %v", b, err)
	}
	return m
}

func TestHTTP_modern_tools_list(t *testing.T) {
	srv := startHTTP(t, config.HTTPConfig{})
	resp := post(t, srv.URL, bytes.NewReader(modernToolsList()), modernHeaders)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if r := result(t, decode(t, resp)); r["resultType"] != "complete" {
		t.Errorf("resultType = %v", r["resultType"])
	}
}

func TestHTTP_cross_origin_request_is_forbidden(t *testing.T) {
	srv := startHTTP(t, config.HTTPConfig{})
	for _, h := range []map[string]string{
		{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"},
		{"Origin": "https://evil.example"},
	} {
		if resp := post(t, srv.URL, bytes.NewReader(modernToolsList()), with(h)); resp.StatusCode != http.StatusForbidden {
			t.Errorf("%v: status = %d, want 403", h, resp.StatusCode)
		}
	}
}

func TestHTTP_trusted_origin_is_allowed(t *testing.T) {
	srv := startHTTP(t, config.HTTPConfig{AllowedOrigins: []string{"https://app.example"}})
	resp := post(t, srv.URL, bytes.NewReader(modernToolsList()),
		with(map[string]string{"Origin": "https://app.example", "Sec-Fetch-Site": "cross-site"}))
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestHTTP_bad_allowed_origin_is_a_config_error(t *testing.T) {
	if _, err := newHTTPHandler(testServer(t), config.HTTPConfig{AllowedOrigins: []string{"not a url"}}); err == nil {
		t.Error("want an error for a malformed origin")
	}
}

func TestHTTP_bearer_token(t *testing.T) {
	srv := startHTTP(t, config.HTTPConfig{AuthToken: testToken})
	cases := []struct {
		auth string
		want int
	}{
		{"", http.StatusUnauthorized},
		{"Bearer wrong-token-wrong-token-wrong", http.StatusUnauthorized},
		{testToken, http.StatusUnauthorized}, // no scheme
		{"Basic " + testToken, http.StatusUnauthorized},
		{"Bearer ", http.StatusUnauthorized},
		{"Bearer " + testToken, http.StatusOK},
		{"bearer " + testToken, http.StatusOK},
	}
	for _, tc := range cases {
		h := with(nil)
		if tc.auth != "" {
			h["Authorization"] = tc.auth
		}
		resp := post(t, srv.URL, bytes.NewReader(modernToolsList()), h)
		if resp.StatusCode != tc.want {
			t.Errorf("Authorization %q: status = %d, want %d", tc.auth, resp.StatusCode, tc.want)
		}
		if tc.want == http.StatusUnauthorized && !strings.HasPrefix(resp.Header.Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("Authorization %q: missing WWW-Authenticate", tc.auth)
		}
	}
}

func TestHTTP_oversized_body_is_rejected(t *testing.T) {
	srv := startHTTP(t, config.HTTPConfig{})
	big := bytes.Repeat([]byte(" "), maxRequestBytes+1)

	resp := post(t, srv.URL, bytes.NewReader(big), modernHeaders)
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("declared length: status = %d, want 413", resp.StatusCode)
	}

	// No Content-Length (chunked): the read is capped and the request fails.
	resp = post(t, srv.URL, io.MultiReader(bytes.NewReader(big)), modernHeaders)
	if resp.StatusCode == http.StatusOK {
		if m := decode(t, resp); m["error"] == nil {
			t.Errorf("chunked oversized body was accepted: %v", m)
		}
	}
}

func TestHTTP_legacy_initialize_is_stateless(t *testing.T) {
	srv := startHTTP(t, config.HTTPConfig{})
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{
		"protocolVersion": "2025-11-25",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "legacy", "version": "1"},
	}})
	resp := post(t, srv.URL, bytes.NewReader(body), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if id := resp.Header.Get("Mcp-Session-Id"); id != "" {
		t.Errorf("stateless server minted a session id %q", id)
	}
	if r := result(t, decode(t, resp)); r["protocolVersion"] != "2025-11-25" {
		t.Errorf("protocolVersion = %v", r["protocolVersion"])
	}
}

func TestHTTP_server_has_timeouts(t *testing.T) {
	s := newHTTPServer("127.0.0.1:0", http.NewServeMux())
	if s.ReadHeaderTimeout == 0 || s.ReadTimeout == 0 || s.IdleTimeout == 0 || s.MaxHeaderBytes == 0 {
		t.Errorf("server = %+v, want every timeout set", s)
	}
}
