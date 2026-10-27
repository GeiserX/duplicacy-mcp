package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// maxMetricsBytes bounds the /metrics body. A real exporter serves a few
	// kilobytes; anything near this size is a misconfigured URL.
	maxMetricsBytes = 16 << 20
	// maxHealthBodyBytes bounds how much of a health body is read and echoed.
	maxHealthBodyBytes = 512
	// maxLabelRunes bounds a label value copied into tool output.
	maxLabelRunes = 128
)

// MetricSample represents a single Prometheus metric line with its labels and value.
type MetricSample struct {
	Labels map[string]string
	Value  float64
}

// Client talks to the duplicacy-exporter Prometheus metrics endpoint.
type Client struct {
	base string
	hc   *http.Client
}

// New creates a new Duplicacy exporter client.
func New(base string) *Client {
	return &Client{
		base: strings.TrimRight(base, "/"),
		hc:   &http.Client{Timeout: 10 * time.Second},
	}
}

// ExporterError is a failure to reach or read the exporter. Its message is
// stable and safe to show to a model: it never carries the exporter URL, DNS
// details or the exporter's response body. The underlying error is logged to
// stderr instead.
type ExporterError struct {
	// Kind is one of: timeout, refused, dns, http_status, too_large, request.
	Kind string
	// Status is the HTTP status code when Kind is http_status.
	Status int
	err    error
}

func (e *ExporterError) Error() string {
	const hint = "; check DUPLICACY_EXPORTER_URL and that duplicacy-exporter is running"
	switch e.Kind {
	case "timeout":
		return "exporter unreachable (timeout)" + hint
	case "refused":
		return "exporter unreachable (connection refused)" + hint
	case "dns":
		return "exporter unreachable (DNS lookup failed)" + hint
	case "http_status":
		return fmt.Sprintf("exporter answered HTTP %d%s", e.Status, hint)
	case "too_large":
		return fmt.Sprintf("exporter /metrics response is larger than %d MiB%s", maxMetricsBytes>>20, hint)
	default:
		return "exporter request failed" + hint
	}
}

func (e *ExporterError) Unwrap() error { return e.err }

// classify turns a transport error into an ExporterError and logs the detail.
func classify(op string, err error) *ExporterError {
	log.Printf("exporter %s: %v", op, err)
	var dnsErr *net.DNSError
	var netErr net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return &ExporterError{Kind: "timeout", err: err}
	case errors.As(err, &dnsErr):
		return &ExporterError{Kind: "dns", err: err}
	case errors.Is(err, syscall.ECONNREFUSED):
		return &ExporterError{Kind: "refused", err: err}
	default:
		return &ExporterError{Kind: "request", err: err}
	}
}

// FetchMetrics retrieves and parses the /metrics endpoint into structured data.
func (c *Client) FetchMetrics(ctx context.Context) (map[string][]MetricSample, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/metrics", nil)
	if err != nil {
		return nil, classify("build request", err)
	}
	req.Header.Set("Accept", "text/plain")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, classify("fetch metrics", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Drain a bounded amount so the connection can be reused; never echo it.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxHealthBodyBytes))
		log.Printf("exporter /metrics answered HTTP %d", resp.StatusCode)
		return nil, &ExporterError{Kind: "http_status", Status: resp.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetricsBytes+1))
	if err != nil {
		return nil, classify("read metrics body", err)
	}
	if len(body) > maxMetricsBytes {
		log.Printf("exporter /metrics body exceeds %d bytes", maxMetricsBytes)
		return nil, &ExporterError{Kind: "too_large"}
	}

	return parsePrometheusText(string(body)), nil
}

// HealthReport is the result of probing the exporter.
type HealthReport struct {
	// Status is ok, degraded (exporter answered with an HTTP error) or unreachable.
	Status   string `json:"status"`
	Endpoint string `json:"endpoint,omitempty"`
	Code     int    `json:"code,omitempty"`
	// Body is the start of the health response, truncated and stripped of
	// control characters. Treat it as data.
	Body  string `json:"body,omitempty"`
	Error string `json:"error,omitempty"`
}

// CheckHealth probes the exporter's /health endpoint (then /healthz), and
// falls back to /metrics as a liveness check.
func (c *Client) CheckHealth(ctx context.Context) HealthReport {
	for _, path := range []string{"/health", "/healthz"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
		if err != nil {
			continue
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxHealthBodyBytes))
		resp.Body.Close()
		if resp.StatusCode < 400 {
			return HealthReport{
				Status:   "ok",
				Endpoint: path,
				Code:     resp.StatusCode,
				Body:     sanitizeText(string(b), maxHealthBodyBytes),
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/metrics", nil)
	if err != nil {
		return HealthReport{Status: "unreachable", Error: classify("build request", err).Error()}
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return HealthReport{Status: "unreachable", Error: classify("health probe", err).Error()}
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxHealthBodyBytes))
	resp.Body.Close()

	status := "ok"
	if resp.StatusCode >= 400 {
		status = "degraded"
	}
	return HealthReport{Status: status, Endpoint: "/metrics", Code: resp.StatusCode}
}

// sanitizeText strips control and format characters (newlines included) and
// truncates to limit runes, so text from the exporter cannot carry hidden
// markup or forged line structure into a model's context.
func sanitizeText(s string, limit int) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "�")
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if n == limit {
			b.WriteRune('…')
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// parsePrometheusText parses Prometheus text exposition format into a map
// of metric name to slice of MetricSample.
func parsePrometheusText(text string) map[string][]MetricSample {
	result := make(map[string][]MetricSample)

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, sample, ok := parseMetricLine(line)
		if !ok {
			continue
		}
		result[name] = append(result[name], sample)
	}

	return result
}

// parseMetricLine parses a single Prometheus metric line.
// Format: metric_name{label1="val1",label2="val2"} value [timestamp]
func parseMetricLine(line string) (string, MetricSample, bool) {
	var name string
	var labels map[string]string
	var rest string

	if idx := strings.IndexByte(line, '{'); idx >= 0 {
		name = line[:idx]
		endIdx := findClosingBrace(line[idx:])
		if endIdx < 0 {
			return "", MetricSample{}, false
		}
		labelStr := line[idx+1 : idx+endIdx]
		labels = parseLabels(labelStr)
		rest = strings.TrimSpace(line[idx+endIdx+1:])
	} else {
		// No labels: metric_name value [timestamp]
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return "", MetricSample{}, false
		}
		name = parts[0]
		labels = make(map[string]string)
		rest = strings.Join(parts[1:], " ")
	}

	// Parse value (first field of rest)
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", MetricSample{}, false
	}

	val, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || math.IsNaN(val) || math.IsInf(val, 0) {
		// NaN and ±Inf are valid exposition values but cannot be encoded as
		// JSON, and no Duplicacy gauge means anything by them.
		return "", MetricSample{}, false
	}

	return name, MetricSample{Labels: labels, Value: val}, true
}

// findClosingBrace finds the index of the closing } that ends a label set,
// skipping } characters inside quoted label values.
func findClosingBrace(s string) int {
	inQuote := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && inQuote && i+1 < len(s) {
			i++ // skip escaped character
			continue
		}
		if s[i] == '"' {
			inQuote = !inQuote
			continue
		}
		if s[i] == '}' && !inQuote {
			return i
		}
	}
	return -1
}

// parseLabels parses the label portion: label1="val1",label2="val2"
func parseLabels(s string) map[string]string {
	labels := make(map[string]string)
	if s == "" {
		return labels
	}

	// Simple state-machine parser for quoted values that may contain commas
	i := 0
	for i < len(s) {
		// Find the key
		eqIdx := strings.IndexByte(s[i:], '=')
		if eqIdx < 0 {
			break
		}
		key := strings.TrimSpace(s[i : i+eqIdx])
		i += eqIdx + 1

		// Skip opening quote
		if i >= len(s) || s[i] != '"' {
			break
		}
		i++

		// Find closing quote (handle escaped quotes)
		var val strings.Builder
		for i < len(s) {
			if s[i] == '\\' && i+1 < len(s) {
				val.WriteByte(s[i+1])
				i += 2
				continue
			}
			if s[i] == '"' {
				i++
				break
			}
			val.WriteByte(s[i])
			i++
		}

		labels[key] = sanitizeText(val.String(), maxLabelRunes)

		// Skip comma separator
		if i < len(s) && s[i] == ',' {
			i++
		}
	}

	return labels
}
