package model

import (
	"errors"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Credentials are write-only in the admin API.
type UpstreamUsageProvider struct {
	ID              int                    `json:"id"`
	BaseURL         string                 `json:"base_url" gorm:"type:text"`
	AccessToken     string                 `json:"-" gorm:"type:text"`
	RefreshToken    string                 `json:"-" gorm:"type:text"`
	IntervalMinutes int                    `json:"interval_minutes"`
	LastPolledAt    int64                  `json:"last_polled_at"`
	Accounts        []UpstreamUsageAccount `json:"accounts" gorm:"foreignKey:ProviderID"`
}

type UpstreamUsageAccount struct {
	ID         int    `json:"id"`
	ProviderID int    `json:"-" gorm:"uniqueIndex:idx_upstream_usage_account"`
	AccountID  int64  `json:"account_id" gorm:"uniqueIndex:idx_upstream_usage_account"`
	GroupsJSON string `json:"-" gorm:"type:text"`
	UsageJSON  string `json:"-" gorm:"type:text"`
	FetchedAt  int64  `json:"fetched_at"`
	LastError  string `json:"last_error" gorm:"type:text"`
}

type UpstreamUsageAccountInput struct {
	AccountID int64    `json:"account_id"`
	Groups    []string `json:"groups"`
}

type UpstreamUsageProviderInput struct {
	BaseURL         string                      `json:"base_url"`
	AccessToken     string                      `json:"access_token"`
	RefreshToken    string                      `json:"refresh_token"`
	IntervalMinutes int                         `json:"interval_minutes"`
	Accounts        []UpstreamUsageAccountInput `json:"accounts"`
}

func (input *UpstreamUsageProviderInput) Validate() error {
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	u, err := url.Parse(input.BaseURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("base_url must be an HTTP(S) URL without credentials, query or fragment")
	}
	if input.IntervalMinutes < 1 || input.IntervalMinutes > 1440 {
		return errors.New("interval_minutes must be between 1 and 1440")
	}
	input.AccessToken = strings.TrimSpace(input.AccessToken)
	input.RefreshToken = strings.TrimSpace(input.RefreshToken)
	if strings.ContainsAny(input.AccessToken+input.RefreshToken, "\r\n") {
		return errors.New("invalid token")
	}
	if len(input.Accounts) > 200 {
		return errors.New("at most 200 accounts per upstream")
	}
	seen := map[int64]bool{}
	for i := range input.Accounts {
		a := &input.Accounts[i]
		if a.AccountID <= 0 || seen[a.AccountID] {
			return errors.New("account_id must be positive and unique within an upstream")
		}
		seen[a.AccountID] = true
		groups := []string{}
		groupSeen := map[string]bool{}
		for _, group := range a.Groups {
			group = strings.TrimSpace(group)
			if group == "" || len(group) > 128 {
				return errors.New("invalid group name")
			}
			if !groupSeen[group] {
				groups = append(groups, group)
				groupSeen[group] = true
			}
		}
		a.Groups = groups
	}
	return nil
}

func ListUpstreamUsageProviders() ([]UpstreamUsageProvider, error) {
	providers := []UpstreamUsageProvider{}
	err := DB.Preload("Accounts", func(db *gorm.DB) *gorm.DB { return db.Order("account_id") }).Order("id").Find(&providers).Error
	return providers, err
}

func GetUpstreamUsageProvider(id int) (*UpstreamUsageProvider, error) {
	var provider UpstreamUsageProvider
	err := DB.Preload("Accounts").First(&provider, id).Error
	return &provider, err
}

func SaveUpstreamUsageProvider(id int, input UpstreamUsageProviderInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		provider := UpstreamUsageProvider{}
		if id > 0 {
			if err := tx.Preload("Accounts").First(&provider, id).Error; err != nil {
				return err
			}
		}
		baseChanged := provider.BaseURL != input.BaseURL
		if baseChanged && (input.AccessToken == "" || input.RefreshToken == "") {
			return errors.New("access_token and refresh_token are required for a new base_url")
		}
		provider.BaseURL = input.BaseURL
		provider.IntervalMinutes = input.IntervalMinutes
		provider.LastPolledAt = 0
		if input.AccessToken != "" {
			provider.AccessToken = input.AccessToken
		}
		if input.RefreshToken != "" {
			provider.RefreshToken = input.RefreshToken
		}
		if err := tx.Omit("Accounts").Save(&provider).Error; err != nil {
			return err
		}
		existing := map[int64]UpstreamUsageAccount{}
		for _, account := range provider.Accounts {
			existing[account.AccountID] = account
		}
		keep := []int{}
		for _, value := range input.Accounts {
			account := existing[value.AccountID]
			account.ProviderID, account.AccountID = provider.ID, value.AccountID
			groups, err := common.Marshal(value.Groups)
			if err != nil {
				return err
			}
			account.GroupsJSON = string(groups)
			if baseChanged {
				account.UsageJSON = ""
				account.FetchedAt = 0
				account.LastError = ""
			}
			if err := tx.Save(&account).Error; err != nil {
				return err
			}
			keep = append(keep, account.ID)
		}
		query := tx.Where("provider_id = ?", provider.ID)
		if len(keep) > 0 {
			query = query.Where("id NOT IN ?", keep)
		}
		return query.Delete(&UpstreamUsageAccount{}).Error
	})
}

func DeleteUpstreamUsageProvider(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider_id = ?", id).Delete(&UpstreamUsageAccount{}).Error; err != nil {
			return err
		}
		return tx.Delete(&UpstreamUsageProvider{}, id).Error
	})
}

func UpdateUpstreamUsageTokens(id int, access, refresh string) error {
	return DB.Model(&UpstreamUsageProvider{}).Where("id = ?", id).Updates(map[string]interface{}{"access_token": access, "refresh_token": refresh}).Error
}

func MarkUpstreamUsagePolled(id int, timestamp int64) error {
	return DB.Model(&UpstreamUsageProvider{}).Where("id = ?", id).Update("last_polled_at", timestamp).Error
}

func SaveUpstreamUsageResult(id int, usage string, fetchedAt int64, message string) error {
	values := map[string]interface{}{"last_error": message}
	if message == "" {
		values["usage_json"] = usage
		values["fetched_at"] = fetchedAt
	}
	return DB.Model(&UpstreamUsageAccount{}).Where("id = ?", id).Updates(values).Error
}
