package common

import (
	_ "embed"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
)

//go:embed request_clients.json
var DefaultRequestClientRules string

type RequestClientDisplay struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	URL  string `json:"url"`
}

type RequestClientRule struct {
	Header  string `json:"header"`
	Pattern string `json:"pattern"`
}

type RequestClientConfig struct {
	RequestClientDisplay
	Enabled bool                `json:"enabled"`
	Rules   []RequestClientRule `json:"rules"`
}

type compiledRequestClient struct {
	display  RequestClientDisplay
	rules    []RequestClientRule
	patterns []*regexp.Regexp
}

var requestClients atomic.Value
var requestClientIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var requestClientHeaderPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

func init() {
	if err := UpdateRequestClientRules(DefaultRequestClientRules); err != nil {
		panic(err)
	}
}

func validClientURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.User == nil
}

func validClientIcon(icon string) bool {
	switch icon {
	case "codex", "codex-desktop", "codex-tui", "codex-vscode", "codex-tui-vscode", "claude-code", "pi", "opencode", "codebuddy", "deepseek-harness":
		return true
	}
	if strings.HasPrefix(icon, "/") && !strings.HasPrefix(icon, "//") && !strings.ContainsAny(icon, "\\\r\n\t") {
		return true
	}
	return validClientURL(icon)
}

func compileRequestClients(value string) ([]compiledRequestClient, error) {
	if len(value) > 256*1024 {
		return nil, fmt.Errorf("client rules exceed 256 KiB")
	}
	var configs []RequestClientConfig
	if err := UnmarshalJsonStr(value, &configs); err != nil {
		return nil, err
	}
	if configs == nil || len(configs) > 100 {
		return nil, fmt.Errorf("client rules must be an array of at most 100 clients")
	}
	clients := make([]compiledRequestClient, 0, len(configs))
	seen := make(map[string]bool)
	for i, config := range configs {
		if !requestClientIDPattern.MatchString(config.ID) || seen[config.ID] {
			return nil, fmt.Errorf("client %d: invalid or duplicate id", i+1)
		}
		seen[config.ID] = true
		if strings.TrimSpace(config.Name) == "" || len(config.Name) > 128 || !validClientIcon(config.Icon) || (config.URL != "" && !validClientURL(config.URL)) {
			return nil, fmt.Errorf("client %s: invalid name, icon or link", config.ID)
		}
		if len(config.Rules) == 0 || len(config.Rules) > 16 {
			return nil, fmt.Errorf("client %s: expected 1 to 16 header rules", config.ID)
		}
		client := compiledRequestClient{display: config.RequestClientDisplay, rules: config.Rules}
		for _, rule := range config.Rules {
			if !requestClientHeaderPattern.MatchString(rule.Header) || rule.Pattern == "" || len(rule.Pattern) > 1024 {
				return nil, fmt.Errorf("client %s: invalid header or pattern", config.ID)
			}
			pattern, err := regexp.Compile(rule.Pattern)
			if err != nil {
				return nil, fmt.Errorf("client %s: %w", config.ID, err)
			}
			client.patterns = append(client.patterns, pattern)
		}
		if config.Enabled {
			clients = append(clients, client)
		}
	}
	return clients, nil
}

func CheckRequestClientRules(value string) error {
	_, err := compileRequestClients(value)
	return err
}

func UpdateRequestClientRules(value string) error {
	clients, err := compileRequestClients(value)
	if err != nil {
		return err
	}
	requestClients.Store(clients)
	return nil
}

// GetRequestClientDisplay exposes only display metadata, never header rules.
func GetRequestClientDisplay() []RequestClientDisplay {
	clients := requestClients.Load().([]compiledRequestClient)
	result := make([]RequestClientDisplay, 0, len(clients))
	for _, client := range clients {
		result = append(result, client.display)
	}
	return result
}

// DetectRequestClient returns the first matching enabled client's stable ID.
// Patterns use Go's bounded-time RE2 engine and are compiled only on updates.
// This is descriptive metadata, never an authentication or routing signal.
func DetectRequestClient(header http.Header) string {
	for _, client := range requestClients.Load().([]compiledRequestClient) {
		for i, rule := range client.rules {
			for _, value := range header.Values(rule.Header) {
				if value != "" && client.patterns[i].MatchString(value) {
					return client.display.ID
				}
			}
		}
	}
	return ""
}
