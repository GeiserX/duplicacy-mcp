package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/mark3labs/mcp-go/server"
)

const fixtureMetrics = `duplicacy_backup_running{snapshot_id="photos",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_exit_code{snapshot_id="photos",storage_target="b2",machine="nas"} 0
duplicacy_backup_last_success_timestamp_seconds{snapshot_id="photos",storage_target="b2",machine="nas"} 1700000000
duplicacy_backup_last_duration_seconds{snapshot_id="photos",storage_target="b2",machine="nas"} 345.67
duplicacy_backup_progress_ratio{snapshot_id="photos",storage_target="b2",machine="nas"} 0.5
duplicacy_backup_running{snapshot_id="docs",storage_target="s3",machine="nas"} 1
duplicacy_prune_running{storage_target="b2",machine="nas"} 0
duplicacy_prune_last_success_timestamp_seconds{storage_target="b2",machine="nas"} 1699999000
duplicacy_storage_total_size_bytes{storage_target="b2",machine="nas"} 1000
duplicacy_storage_total_chunks{storage_target="b2",machine="nas"} 10
duplicacy_snapshot_revisions{snapshot_id="photos",storage_target="b2",machine="nas"} 7
duplicacy_poller_last_success_timestamp_seconds 1700000500
duplicacy_poller_errors_total 0
duplicacy_exporter_last_activity_timestamp_seconds 1700000600
duplicacy_exporter_backups_seen_total 4
`

func testServer(t *testing.T) *server.MCPServer {
	t.Helper()
	exporter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fixtureMetrics))
	}))
	t.Cleanup(exporter.Close)
	return newServer(client.New(exporter.URL))
}

// modernMeta is the per-request _meta a 2026-07-28 client sends instead of
// an initialize handshake.
func modernMeta() map[string]any {
	return map[string]any{
		"io.modelcontextprotocol/protocolVersion":    "2026-07-28",
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "smoke", "version": "1"},
	}
}

// stdio drives the real stdio transport over pipes.
type stdio struct {
	t   *testing.T
	in  *io.PipeWriter
	out chan map[string]any
}

func startStdio(t *testing.T, s *server.MCPServer) *stdio {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	srv := server.NewStdioServer(s)
	srv.SetErrorLogger(log.New(io.Discard, "", 0))
	go func() { _ = srv.Listen(ctx, inR, outW) }()
	out := make(chan map[string]any, 16)
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var msg map[string]any
			if json.Unmarshal(sc.Bytes(), &msg) == nil {
				out <- msg
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		inW.Close()
		outW.Close()
	})
	return &stdio{t: t, in: inW, out: out}
}

func (s *stdio) send(msg map[string]any) {
	b, err := json.Marshal(msg)
	if err != nil {
		s.t.Fatal(err)
	}
	go func() { _, _ = s.in.Write(append(b, '\n')) }()
}

// request sends one JSON-RPC request and returns the response with its id.
func (s *stdio) request(id int, method string, params map[string]any) map[string]any {
	s.t.Helper()
	msg := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		msg["params"] = params
	}
	s.send(msg)
	deadline := time.After(5 * time.Second)
	for {
		select {
		case resp := <-s.out:
			if got, _ := resp["id"].(float64); int(got) == id {
				return resp
			}
		case <-deadline:
			s.t.Fatalf("no response to %s", method)
		}
	}
}

func modern(params map[string]any) map[string]any {
	if params == nil {
		params = map[string]any{}
	}
	params["_meta"] = modernMeta()
	return params
}

func result(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	r, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result in %v", resp)
	}
	return r
}

func TestProtocol_modern_server_discover(t *testing.T) {
	sess := startStdio(t, testServer(t))
	r := result(t, sess.request(1, "server/discover", modern(nil)))

	var versions []string
	for _, v := range r["supportedVersions"].([]any) {
		versions = append(versions, v.(string))
	}
	for _, want := range []string{"2026-07-28", "2025-11-25", "2024-11-05"} {
		if !slices.Contains(versions, want) {
			t.Errorf("supportedVersions %v lacks %s", versions, want)
		}
	}
	if r["instructions"] == "" || r["instructions"] == nil {
		t.Error("discover carries no instructions")
	}
	caps := r["capabilities"].(map[string]any)
	if res, _ := caps["resources"].(map[string]any); res["subscribe"] == true || res["listChanged"] == true {
		t.Errorf("resources capability advertises what the server never sends: %v", res)
	}
	info, _ := r["_meta"].(map[string]any)["io.modelcontextprotocol/serverInfo"].(map[string]any)
	if info["title"] != serverTitle || info["websiteUrl"] != websiteURL {
		t.Errorf("serverInfo = %v", info)
	}
}

func TestProtocol_modern_tools_list_is_cacheable(t *testing.T) {
	sess := startStdio(t, testServer(t))
	r := result(t, sess.request(1, "tools/list", modern(nil)))
	if r["resultType"] != "complete" {
		t.Errorf("resultType = %v, want complete", r["resultType"])
	}
	if r["ttlMs"] != float64(listCacheTTLMs) || r["cacheScope"] != "public" {
		t.Errorf("ttlMs = %v cacheScope = %v, want %d public", r["ttlMs"], r["cacheScope"], listCacheTTLMs)
	}
	if n := len(r["tools"].([]any)); n != 6 {
		t.Errorf("tools = %d, want 6", n)
	}
}

func TestProtocol_modern_resources_read_is_not_cached(t *testing.T) {
	sess := startStdio(t, testServer(t))
	r := result(t, sess.request(1, "resources/read", modern(map[string]any{"uri": "duplicacy://status"})))
	if r["ttlMs"] != float64(0) || r["cacheScope"] != "private" {
		t.Errorf("ttlMs = %v cacheScope = %v, want 0 private", r["ttlMs"], r["cacheScope"])
	}
}

func TestProtocol_modern_ping_is_removed(t *testing.T) {
	sess := startStdio(t, testServer(t))
	resp := sess.request(1, "ping", modern(nil))
	e, _ := resp["error"].(map[string]any)
	if e["code"] != float64(-32601) {
		t.Errorf("ping answered %v, want error -32601", resp)
	}
}

func TestProtocol_legacy_initialize_still_works(t *testing.T) {
	for _, version := range []string{"2025-11-25", "2025-06-18", "2024-11-05"} {
		t.Run(version, func(t *testing.T) {
			sess := startStdio(t, testServer(t))
			r := result(t, sess.request(1, "initialize", map[string]any{
				"protocolVersion": version,
				"capabilities":    map[string]any{},
				"clientInfo":      map[string]any{"name": "legacy", "version": "1"},
			}))
			if r["protocolVersion"] != version {
				t.Errorf("protocolVersion = %v, want %s", r["protocolVersion"], version)
			}
			sess.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
			list := result(t, sess.request(2, "tools/list", nil))
			if n := len(list["tools"].([]any)); n != 6 {
				t.Errorf("tools = %d, want 6", n)
			}
			if _, ok := list["resultType"]; ok {
				t.Error("a legacy tools/list result should not carry resultType")
			}
		})
	}
}

// Every tool's real result must satisfy its own outputSchema: the server
// runs with output validation on, which turns a mismatch into isError.
func TestProtocol_every_tool_result_matches_its_output_schema(t *testing.T) {
	sess := startStdio(t, testServer(t))
	args := map[string]map[string]any{
		"get_backup_status":  {"snapshot_id": "photos"},
		"get_backup_history": {"snapshot_id": "docs"},
	}
	list := result(t, sess.request(1, "tools/list", modern(nil)))
	for i, raw := range list["tools"].([]any) {
		name := raw.(map[string]any)["name"].(string)
		r := result(t, sess.request(10+i, "tools/call", modern(map[string]any{"name": name, "arguments": args[name]})))
		if r["isError"] == true {
			t.Errorf("%s: %v", name, r["content"])
			continue
		}
		if r["resultType"] != "complete" || r["structuredContent"] == nil {
			t.Errorf("%s: resultType = %v, structuredContent = %v", name, r["resultType"], r["structuredContent"])
		}
	}
}

func TestProtocol_unknown_argument_is_rejected(t *testing.T) {
	sess := startStdio(t, testServer(t))
	r := result(t, sess.request(1, "tools/call", modern(map[string]any{
		"name":      "list_snapshots",
		"arguments": map[string]any{"bogus": 1},
	})))
	if r["isError"] != true {
		t.Errorf("an unknown argument should be a tool error, got %v", r)
	}
}
