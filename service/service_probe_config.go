package service

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"sync"
	"unicode"

	"github.com/QuantumNous/new-api/model"
)

var serviceProbeConfigMutex sync.Mutex

type ServiceProbeConfigView struct {
	Enabled       bool   `json:"enabled"`
	URL           string `json:"url"`
	KeyConfigured bool   `json:"key_configured"`
}

func resolveServiceProbeConfig() (model.ServiceProbeConfig, error) {
	config, exists, err := model.GetServiceProbeConfig()
	if err != nil {
		return config, errors.New("serviceProbe.invalidConfig")
	}
	if !exists {
		config.URL = strings.TrimSpace(os.Getenv("GREATMAO_HIVE_URL"))
		config.ReadAPIKey = strings.TrimSpace(os.Getenv("GREATMAO_HIVE_READ_API_KEY"))
		config.Enabled = config.URL != "" && config.ReadAPIKey != ""
	}
	return config, nil
}

func GetServiceProbeConfigView() (ServiceProbeConfigView, error) {
	config, err := resolveServiceProbeConfig()
	return ServiceProbeConfigView{config.Enabled, config.URL, config.ReadAPIKey != ""}, err
}

func validServiceProbeURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Hostname() != "" && (u.Scheme == "http" || u.Scheme == "https") && u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.Contains(value, "#")
}

// Empty key input retains the effective key (including legacy environment configuration).
func SaveServiceProbeSettings(enabled bool, baseURL, newKey string) (ServiceProbeConfigView, error) {
	serviceProbeConfigMutex.Lock()
	defer serviceProbeConfigMutex.Unlock()
	config, err := resolveServiceProbeConfig()
	if err != nil {
		return ServiceProbeConfigView{}, err
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	newKey = strings.TrimSpace(newKey)
	if baseURL != "" && (!validServiceProbeURL(baseURL) || len(baseURL) > 2048) {
		return ServiceProbeConfigView{}, errors.New("serviceProbe.invalidConfig")
	}
	if newKey != "" {
		if len(newKey) < 32 || len(newKey) > 4096 || strings.IndexFunc(newKey, unicode.IsControl) >= 0 {
			return ServiceProbeConfigView{}, errors.New("serviceProbe.settings.invalidKey")
		}
		config.ReadAPIKey = newKey
	}
	if enabled && (baseURL == "" || config.ReadAPIKey == "") {
		return ServiceProbeConfigView{}, errors.New("serviceProbe.settings.required")
	}
	config.Enabled, config.URL = enabled, baseURL
	if err := model.SaveServiceProbeConfig(config); err != nil {
		return ServiceProbeConfigView{}, errors.New("serviceProbe.settings.saveFailed")
	}
	return ServiceProbeConfigView{config.Enabled, config.URL, config.ReadAPIKey != ""}, nil
}
