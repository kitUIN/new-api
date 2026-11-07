package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var ErrServiceProbeNotConfigured = errors.New("serviceProbe.notConfigured")

// Never forward the read key through a redirect, even to the same host.
var serviceProbeClient = &http.Client{
	Timeout:       6 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
}

// ReadServiceProbe exposes only the three read-only Hive resources used by the dashboard.
// A nil result denotes an unavailable live sample, never zero traffic.
func ReadServiceProbe(ctx context.Context, resource, id, window string) (json.RawMessage, error) {
	config, err := resolveServiceProbeConfig()
	if err != nil {
		return nil, err
	}
	baseURL, key := config.URL, config.ReadAPIKey
	if !config.Enabled || baseURL == "" || key == "" {
		return nil, ErrServiceProbeNotConfigured
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("serviceProbe.invalidConfig")
	}
	path := "/api/v1/bees/traffic/quotas"
	query := url.Values{}
	if resource != "quotas" {
		if strings.TrimSpace(id) == "" || len([]rune(id)) > 128 || id == "." || id == ".." || strings.ContainsAny(id, "/\\\r\n") {
			return nil, errors.New("serviceProbe.invalidRequest")
		}
		path = "/api/v1/bees/" + url.PathEscape(id)
		switch resource {
		case "current":
			path += "/traffic/current"
		case "history":
			duration, ok := map[string]time.Duration{"1h": time.Hour, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour}[window]
			if !ok {
				return nil, errors.New("serviceProbe.invalidRequest")
			}
			// Allow a small clock/transport margin; Hive forbids future windows.
			end := time.Now().UTC().Add(-5 * time.Second)
			query.Set("from", end.Add(-duration).Format(time.RFC3339Nano))
			query.Set("to", end.Format(time.RFC3339Nano))
			path += "/online/history"
		default:
			return nil, errors.New("serviceProbe.invalidRequest")
		}
	}
	endpoint := strings.TrimRight(u.String(), "/") + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.New("serviceProbe.invalidConfig")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	response, err := serviceProbeClient.Do(req)
	if err != nil {
		return nil, errors.New("serviceProbe.connectionFailed")
	}
	defer response.Body.Close()
	if resource == "current" && (response.StatusCode == 503 || response.StatusCode == 504 || response.StatusCode == 404) {
		return nil, nil
	}
	if response.StatusCode != http.StatusOK {
		// Do not expose upstream URLs, credentials, or error bodies to the browser.
		return nil, errors.New("serviceProbe.upstreamFailed")
	}
	const maxResponseBytes = 4 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return nil, errors.New("serviceProbe.invalidResponse")
	}
	var payload map[string]json.RawMessage
	if err := common.Unmarshal(body, &payload); err != nil || payload == nil {
		return nil, errors.New("serviceProbe.invalidResponse")
	}
	return json.RawMessage(body), nil
}
