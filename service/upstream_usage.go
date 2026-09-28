package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type UpstreamUsageStats struct {
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	Cost         float64 `json:"cost"`
	StandardCost float64 `json:"standard_cost"`
}

type UpstreamUsageWindow struct {
	Utilization      *float64            `json:"utilization"`
	ResetsAt         string              `json:"resets_at"`
	RemainingSeconds int64               `json:"remaining_seconds"`
	WindowStats      *UpstreamUsageStats `json:"window_stats"`
}

type UpstreamUsageData struct {
	UpdatedAt string               `json:"updated_at"`
	FiveHour  *UpstreamUsageWindow `json:"five_hour"`
	SevenDay  *UpstreamUsageWindow `json:"seven_day"`
}

type UpstreamUsageAccountView struct {
	model.UpstreamUsageAccount
	Groups    []string           `json:"groups"`
	Usage     *UpstreamUsageData `json:"usage"`
	Available bool               `json:"available"`
}

type UpstreamUsageProviderView struct {
	ID              int                        `json:"id"`
	BaseURL         string                     `json:"base_url"`
	IntervalMinutes int                        `json:"interval_minutes"`
	LastPolledAt    int64                      `json:"last_polled_at"`
	Accounts        []UpstreamUsageAccountView `json:"accounts"`
}

func ListUpstreamUsage() ([]UpstreamUsageProviderView, error) {
	providers, err := model.ListUpstreamUsageProviders()
	if err != nil {
		return nil, err
	}
	result := []UpstreamUsageProviderView{}
	for _, p := range providers {
		view := UpstreamUsageProviderView{ID: p.ID, BaseURL: p.BaseURL, IntervalMinutes: p.IntervalMinutes, LastPolledAt: p.LastPolledAt, Accounts: []UpstreamUsageAccountView{}}
		for _, a := range p.Accounts {
			account := UpstreamUsageAccountView{UpstreamUsageAccount: a, Groups: []string{}}
			_ = common.UnmarshalJsonStr(a.GroupsJSON, &account.Groups)
			if a.UsageJSON != "" {
				_ = common.UnmarshalJsonStr(a.UsageJSON, &account.Usage)
			}
			account.Available = account.Usage != nil && a.LastError == "" && a.FetchedAt > 0 && time.Now().Unix()-a.FetchedAt <= int64(p.IntervalMinutes*120+60)
			view.Accounts = append(view.Accounts, account)
		}
		result = append(result, view)
	}
	return result, nil
}

// Serialize refresh, edits and deletion for the same upstream. Rotating refresh
// tokens must never be used concurrently by the scheduler and manual refresh.
var upstreamUsageLocks [64]sync.Mutex

func upstreamUsageLock(id int) *sync.Mutex {
	return &upstreamUsageLocks[uint(id)%uint(len(upstreamUsageLocks))]
}

func SaveUpstreamUsage(id int, input model.UpstreamUsageProviderInput) error {
	lock := upstreamUsageLock(id)
	lock.Lock()
	defer lock.Unlock()
	return model.SaveUpstreamUsageProvider(id, input)
}

func DeleteUpstreamUsage(id int) error {
	lock := upstreamUsageLock(id)
	lock.Lock()
	defer lock.Unlock()
	return model.DeleteUpstreamUsageProvider(id)
}

var upstreamUsageHTTPClient = &http.Client{
	Timeout:       30 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// Do not return response bodies or transport errors: either can contain secrets.
func upstreamUsageRequest(ctx context.Context, p *model.UpstreamUsageProvider, path string, payload []byte, target interface{}) (int, error) {
	method := http.MethodGet
	if payload != nil {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, p.BaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return 0, errors.New("invalid upstream request")
	}
	req.Header.Set("Accept", "application/json")
	if payload == nil {
		req.Header.Set("Authorization", "Bearer "+p.AccessToken)
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := upstreamUsageHTTPClient.Do(req)
	if err != nil {
		return 0, errors.New("upstream request failed or timed out")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return res.StatusCode, fmt.Errorf("upstream HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 1024*1024+1))
	if err != nil || len(body) > 1024*1024 {
		return res.StatusCode, errors.New("invalid upstream response size")
	}
	if err := common.Unmarshal(body, target); err != nil {
		return res.StatusCode, errors.New("invalid upstream JSON")
	}
	return res.StatusCode, nil
}

func refreshUpstreamUsageToken(ctx context.Context, p *model.UpstreamUsageProvider) error {
	payload, err := common.Marshal(map[string]string{"refresh_token": p.RefreshToken})
	if err != nil {
		return err
	}
	var response struct {
		Code *int `json:"code"`
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if _, err := upstreamUsageRequest(ctx, p, "/api/v1/auth/refresh", payload, &response); err != nil {
		return err
	}
	if response.Code == nil || *response.Code != 0 || response.Data.AccessToken == "" || response.Data.RefreshToken == "" {
		return errors.New("upstream token refresh failed")
	}
	if err := model.UpdateUpstreamUsageTokens(p.ID, response.Data.AccessToken, response.Data.RefreshToken); err != nil {
		return err
	}
	p.AccessToken, p.RefreshToken = response.Data.AccessToken, response.Data.RefreshToken
	return nil
}

func fetchUpstreamAccountUsage(ctx context.Context, p *model.UpstreamUsageProvider, accountID int64, canRefresh *bool) (*UpstreamUsageData, error) {
	var response struct {
		Code *int               `json:"code"`
		Data *UpstreamUsageData `json:"data"`
	}
	path := fmt.Sprintf("/api/v1/admin/accounts/%d/usage?timezone=Asia%%2FShanghai", accountID)
	status, err := upstreamUsageRequest(ctx, p, path, nil, &response)
	if status == http.StatusUnauthorized && *canRefresh {
		*canRefresh = false
		if err := refreshUpstreamUsageToken(ctx, p); err != nil {
			return nil, err
		}
		_, err = upstreamUsageRequest(ctx, p, path, nil, &response)
	}
	if err != nil {
		return nil, err
	}
	if response.Code == nil || *response.Code != 0 || response.Data == nil {
		return nil, errors.New("upstream usage unavailable")
	}
	if response.Data.SevenDay == nil || response.Data.SevenDay.Utilization == nil {
		return nil, errors.New("upstream seven-day usage missing")
	}
	for _, window := range []*UpstreamUsageWindow{response.Data.FiveHour, response.Data.SevenDay} {
		if window != nil && (window.Utilization == nil || *window.Utilization < 0 || *window.Utilization > 100) {
			return nil, errors.New("invalid upstream utilization")
		}
	}
	return response.Data, nil
}

func PollUpstreamUsage(ctx context.Context, id int, dueOnly bool) error {
	lock := upstreamUsageLock(id)
	if !lock.TryLock() {
		if dueOnly {
			return nil
		}
		return errors.New("upstream is busy; try again shortly")
	}
	defer lock.Unlock()
	p, err := model.GetUpstreamUsageProvider(id)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	if dueOnly && now-p.LastPolledAt < int64(p.IntervalMinutes*60) {
		return nil
	}
	if err := model.MarkUpstreamUsagePolled(id, now); err != nil {
		return err
	}
	canRefresh := true
	for _, account := range p.Accounts {
		usage, fetchErr := fetchUpstreamAccountUsage(ctx, p, account.AccountID, &canRefresh)
		message, data := "", ""
		if fetchErr != nil {
			message = fetchErr.Error()
		} else {
			encoded, err := common.Marshal(usage)
			if err != nil {
				return err
			}
			data = string(encoded)
		}
		if err := model.SaveUpstreamUsageResult(account.ID, data, time.Now().Unix(), message); err != nil {
			return err
		}
	}
	return nil
}

func StartUpstreamUsageTask() {
	if !common.IsMasterNode {
		return
	}
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		workers := make(chan struct{}, 4)
		for {
			providers, err := model.ListUpstreamUsageProviders()
			if err != nil {
				common.SysError("upstream usage: failed to load configuration")
			} else {
				for _, provider := range providers {
					if time.Now().Unix()-provider.LastPolledAt < int64(provider.IntervalMinutes*60) {
						continue
					}
					select {
					case workers <- struct{}{}:
						go func(id int) {
							defer func() { <-workers }()
							if err := PollUpstreamUsage(context.Background(), id, true); err != nil {
								common.SysError("upstream usage: collection failed")
							}
						}(provider.ID)
					default:
					}
				}
			}
			<-ticker.C
		}
	}()
}
