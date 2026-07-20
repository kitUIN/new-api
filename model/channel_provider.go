package model

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"gorm.io/gorm"
)

type ChannelProvider struct {
	Id                 int     `json:"id"`
	Name               string  `json:"name" gorm:"size:128;not null"`
	BaseURL            string  `json:"base_url" gorm:"size:255;not null;uniqueIndex"`
	Status             int     `json:"status" gorm:"default:1"`
	Balance            float64 `json:"balance"`
	BalanceUpdatedTime int64   `json:"balance_updated_time" gorm:"bigint"`
	CreatedTime        int64   `json:"created_time" gorm:"bigint"`
	UpdatedTime        int64   `json:"updated_time" gorm:"bigint"`
	Remark             string  `json:"remark" gorm:"type:varchar(255)"`
	Settings           string  `json:"settings" gorm:"column:settings;type:text"`
}

type ChannelProviderSummary struct {
	Id                 int     `json:"id"`
	Name               string  `json:"name"`
	BaseURL            string  `json:"base_url"`
	Status             int     `json:"status"`
	Balance            float64 `json:"balance"`
	BalanceUpdatedTime int64   `json:"balance_updated_time"`
	Settings           string  `json:"settings"`
}

type ChannelProviderTree struct {
	Key                string     `json:"key"`
	Id                 string     `json:"id"`
	IsProvider         bool       `json:"is_provider"`
	ProviderID         int        `json:"provider_id"`
	Name               string     `json:"name"`
	BaseURL            string     `json:"base_url"`
	Status             int        `json:"status"`
	Group              string     `json:"group"`
	Settings           string     `json:"settings"`
	Balance            float64    `json:"balance"`
	BalanceUpdatedTime int64      `json:"balance_updated_time"`
	UsedQuota          int64      `json:"used_quota"`
	ResponseTime       int        `json:"response_time"`
	Priority           any        `json:"priority"`
	Weight             any        `json:"weight"`
	ChannelCount       int        `json:"channel_count"`
	EnabledCount       int        `json:"enabled_count"`
	Children           []*Channel `json:"children"`
}

func NormalizeChannelProviderBaseURL(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return ""
	}
	return strings.TrimRight(trimmed, "/")
}

func EffectiveBaseURLForChannel(channel *Channel) string {
	if channel == nil {
		return ""
	}
	if channel.BaseURL != nil {
		if normalized := NormalizeChannelProviderBaseURL(*channel.BaseURL); normalized != "" {
			return normalized
		}
	}
	return NormalizeChannelProviderBaseURL(constant.ChannelBaseURLs[channel.Type])
}

func defaultChannelProviderName(baseURL string) string {
	parsed, err := url.Parse(baseURL)
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}
	if baseURL == "" {
		return "未设置地址"
	}
	return baseURL
}

func providerDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return DB
}

func GetOrCreateChannelProviderByBaseURL(tx *gorm.DB, baseURL string) (*ChannelProvider, error) {
	normalized := NormalizeChannelProviderBaseURL(baseURL)
	if normalized == "" {
		return nil, errors.New("供应商 API 地址不能为空")
	}

	db := providerDB(tx)
	var provider ChannelProvider
	err := db.Where("base_url = ?", normalized).First(&provider).Error
	if err == nil {
		return &provider, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	now := common.GetTimestamp()
	provider = ChannelProvider{
		Name:        defaultChannelProviderName(normalized),
		BaseURL:     normalized,
		Status:      common.ChannelStatusEnabled,
		CreatedTime: now,
		UpdatedTime: now,
	}
	if err := db.Create(&provider).Error; err != nil {
		// A concurrent creator may have inserted the same base URL first.
		if retryErr := db.Where("base_url = ?", normalized).First(&provider).Error; retryErr == nil {
			return &provider, nil
		}
		return nil, err
	}
	return &provider, nil
}

func GetChannelProviderByID(id int) (*ChannelProvider, error) {
	var provider ChannelProvider
	if err := DB.First(&provider, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func (provider *ChannelProvider) GetOtherSettings() dto.ChannelProviderSettings {
	setting := dto.ChannelProviderSettings{}
	if provider == nil || provider.Settings == "" {
		return setting
	}
	if err := common.UnmarshalJsonStr(provider.Settings, &setting); err != nil {
		common.SysLog(fmt.Sprintf("failed to unmarshal provider settings: provider_id=%d, error=%v", provider.Id, err))
		provider.Settings = "{}"
		_ = DB.Model(&ChannelProvider{}).Where("id = ?", provider.Id).Update("settings", provider.Settings).Error
	}
	return setting
}

func (provider *ChannelProvider) SetOtherSettings(setting dto.ChannelProviderSettings) {
	settingBytes, err := common.Marshal(setting)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to marshal provider settings: provider_id=%d, error=%v", provider.Id, err))
		return
	}
	provider.Settings = string(settingBytes)
}

func EnsureProviderForChannel(tx *gorm.DB, channel *Channel) (*ChannelProvider, error) {
	if channel == nil {
		return nil, errors.New("channel is nil")
	}
	db := providerDB(tx)
	if channel.ProviderID > 0 {
		provider, err := GetChannelProviderByIDWithDB(db, channel.ProviderID)
		if err != nil {
			return nil, err
		}
		baseURL := provider.BaseURL
		channel.BaseURL = &baseURL
		return provider, nil
	}
	baseURL := EffectiveBaseURLForChannel(channel)
	if baseURL == "" {
		return nil, nil
	}
	provider, err := GetOrCreateChannelProviderByBaseURL(db, baseURL)
	if err != nil {
		return nil, err
	}
	channel.ProviderID = provider.Id
	baseURL = provider.BaseURL
	channel.BaseURL = &baseURL
	return provider, nil
}

func GetChannelProviderByIDWithDB(db *gorm.DB, id int) (*ChannelProvider, error) {
	var provider ChannelProvider
	if err := db.First(&provider, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func AttachChannelProviderSummaries(channels []*Channel) {
	if len(channels) == 0 {
		return
	}
	idSet := make(map[int]struct{})
	for _, channel := range channels {
		if channel != nil && channel.ProviderID > 0 {
			idSet[channel.ProviderID] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return
	}
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	var providers []ChannelProvider
	if err := DB.Where("id in (?)", ids).Find(&providers).Error; err != nil {
		common.SysLog(fmt.Sprintf("failed to attach channel providers: %v", err))
		return
	}
	providerMap := make(map[int]*ChannelProviderSummary, len(providers))
	for i := range providers {
		provider := providers[i]
		providerMap[provider.Id] = &ChannelProviderSummary{
			Id:                 provider.Id,
			Name:               provider.Name,
			BaseURL:            provider.BaseURL,
			Status:             provider.Status,
			Balance:            provider.Balance,
			BalanceUpdatedTime: provider.BalanceUpdatedTime,
			Settings:           provider.Settings,
		}
	}
	for _, channel := range channels {
		if channel != nil {
			channel.Provider = providerMap[channel.ProviderID]
		}
	}
}

func MigrateChannelProviders() error {
	var channels []*Channel
	if err := DB.Find(&channels).Error; err != nil {
		return err
	}
	if len(channels) == 0 {
		return nil
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		for _, channel := range channels {
			baseURL := EffectiveBaseURLForChannel(channel)
			if baseURL == "" {
				continue
			}
			provider, err := GetOrCreateChannelProviderByBaseURL(tx, baseURL)
			if err != nil {
				return err
			}
			updates := map[string]interface{}{}
			if channel.ProviderID != provider.Id {
				updates["provider_id"] = provider.Id
			}
			if channel.BaseURL == nil || *channel.BaseURL != provider.BaseURL {
				updates["base_url"] = provider.BaseURL
			}
			if len(updates) == 0 {
				continue
			}
			if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return MigrateChannelProviderQuerySettings()
}

func MigrateChannelProviderQuerySettings() error {
	var providers []*ChannelProvider
	if err := DB.Find(&providers).Error; err != nil {
		return err
	}
	for _, provider := range providers {
		settings := provider.GetOtherSettings()
		if settings.BalanceQuery.Enabled && settings.GroupQuery.Enabled {
			if err := ClearChannelQuerySettingsForProvider(provider.Id); err != nil {
				return err
			}
			continue
		}
		var channels []*Channel
		if err := DB.Where("provider_id = ?", provider.Id).Order("id asc").Find(&channels).Error; err != nil {
			return err
		}
		changed := false
		for _, channel := range channels {
			channelSettings := channel.GetOtherSettings()
			if !settings.BalanceQuery.Enabled && channelSettings.BalanceQuery.Enabled {
				sourceChannel := channel
				sourceSettings := channelSettings
				if channelSettings.BalanceQuery.SourceChannelID > 0 && channelSettings.BalanceQuery.SourceChannelID != channel.Id {
					if source, err := GetChannelById(channelSettings.BalanceQuery.SourceChannelID, true); err == nil && source.ProviderID == provider.Id {
						sourceChannel = source
						sourceSettings = source.GetOtherSettings()
					}
				}
				if sourceSettings.BalanceQuery.Enabled {
					settings.BalanceQuery = sourceSettings.BalanceQuery
					settings.BalanceQuery.SourceChannelID = sourceChannel.Id
					provider.Balance = sourceChannel.Balance
					provider.BalanceUpdatedTime = sourceChannel.BalanceUpdatedTime
					changed = true
				}
			}
			if !settings.GroupQuery.Enabled && channelSettings.GroupQuery.Enabled {
				sourceChannel := channel
				sourceSettings := channelSettings
				if channelSettings.GroupQuery.SourceChannelID > 0 && channelSettings.GroupQuery.SourceChannelID != channel.Id {
					if source, err := GetChannelById(channelSettings.GroupQuery.SourceChannelID, true); err == nil && source.ProviderID == provider.Id {
						sourceChannel = source
						sourceSettings = source.GetOtherSettings()
					}
				}
				if sourceSettings.GroupQuery.Enabled {
					settings.GroupQuery = sourceSettings.GroupQuery
					settings.GroupQuery.SourceChannelID = sourceChannel.Id
					changed = true
				}
			}
			if settings.BalanceQuery.Enabled && settings.GroupQuery.Enabled {
				break
			}
		}
		if !changed {
			if settings.BalanceQuery.Enabled || settings.GroupQuery.Enabled {
				if err := ClearChannelQuerySettingsForProvider(provider.Id); err != nil {
					return err
				}
			}
			continue
		}
		provider.SetOtherSettings(settings)
		if err := DB.Model(&ChannelProvider{}).Where("id = ?", provider.Id).Updates(map[string]interface{}{
			"settings":             provider.Settings,
			"balance":              provider.Balance,
			"balance_updated_time": provider.BalanceUpdatedTime,
			"updated_time":         common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		if settings.BalanceQuery.Enabled || settings.GroupQuery.Enabled {
			if err := ClearChannelQuerySettingsForProvider(provider.Id); err != nil {
				return err
			}
		}
	}
	return nil
}

func ClearChannelQuerySettingsForProvider(providerID int) error {
	if providerID <= 0 {
		return nil
	}
	var channels []*Channel
	if err := DB.Where("provider_id = ?", providerID).Find(&channels).Error; err != nil {
		return err
	}
	for _, channel := range channels {
		if channel == nil || strings.TrimSpace(channel.OtherSettings) == "" {
			continue
		}
		settings := map[string]interface{}{}
		if err := common.UnmarshalJsonStr(channel.OtherSettings, &settings); err != nil {
			common.SysLog(fmt.Sprintf("failed to unmarshal channel settings while clearing query settings: channel_id=%d, error=%v", channel.Id, err))
			continue
		}
		_, hasBalanceQuery := settings["balance_query"]
		_, hasGroupQuery := settings["group_query"]
		if !hasBalanceQuery && !hasGroupQuery {
			continue
		}
		delete(settings, "balance_query")
		delete(settings, "group_query")
		data, err := common.Marshal(settings)
		if err != nil {
			return err
		}
		channel.OtherSettings = string(data)
		if err := DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("settings", channel.OtherSettings).Error; err != nil {
			return err
		}
	}
	return nil
}

func StripChannelQuerySettings(otherSettings string) (string, bool, error) {
	if strings.TrimSpace(otherSettings) == "" {
		return otherSettings, false, nil
	}
	settings := map[string]interface{}{}
	if err := common.UnmarshalJsonStr(otherSettings, &settings); err != nil {
		return otherSettings, false, err
	}
	_, hasBalanceQuery := settings["balance_query"]
	_, hasGroupQuery := settings["group_query"]
	if !hasBalanceQuery && !hasGroupQuery {
		return otherSettings, false, nil
	}
	delete(settings, "balance_query")
	delete(settings, "group_query")
	data, err := common.Marshal(settings)
	if err != nil {
		return otherSettings, false, err
	}
	return string(data), true, nil
}

func ListChannelProviders(offset int, limit int) ([]*ChannelProvider, int64, error) {
	var total int64
	if err := DB.Model(&ChannelProvider{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var providers []*ChannelProvider
	query := DB.Order("id desc").Offset(offset)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&providers).Error; err != nil {
		return nil, 0, err
	}
	return providers, total, nil
}

func SearchChannelProviders(keyword string, offset int, limit int) ([]*ChannelProvider, int64, error) {
	db := DB.Model(&ChannelProvider{})
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR base_url LIKE ? OR remark LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var providers []*ChannelProvider
	query := db.Order("id desc").Offset(offset)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&providers).Error; err != nil {
		return nil, 0, err
	}
	return providers, total, nil
}

func UpdateChannelProvider(provider *ChannelProvider) error {
	if provider == nil || provider.Id == 0 {
		return errors.New("缺少供应商 ID")
	}
	provider.BaseURL = NormalizeChannelProviderBaseURL(provider.BaseURL)
	if provider.BaseURL == "" {
		return errors.New("供应商 API 地址不能为空")
	}
	if strings.TrimSpace(provider.Name) == "" {
		provider.Name = defaultChannelProviderName(provider.BaseURL)
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var origin ChannelProvider
		if err := tx.First(&origin, "id = ?", provider.Id).Error; err != nil {
			return err
		}
		provider.CreatedTime = origin.CreatedTime
		provider.UpdatedTime = common.GetTimestamp()
		if provider.Settings == "" {
			provider.Settings = origin.Settings
		}
		if err := tx.Model(&ChannelProvider{}).Where("id = ?", provider.Id).Updates(map[string]interface{}{
			"name":         provider.Name,
			"base_url":     provider.BaseURL,
			"status":       provider.Status,
			"updated_time": provider.UpdatedTime,
			"remark":       provider.Remark,
			"settings":     provider.Settings,
		}).Error; err != nil {
			return err
		}
		if origin.BaseURL != provider.BaseURL {
			if err := tx.Model(&Channel{}).Where("provider_id = ?", provider.Id).Update("base_url", provider.BaseURL).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func DeleteChannelProvider(id int) error {
	var count int64
	if err := DB.Model(&Channel{}).Where("provider_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该供应商下仍有 %d 个渠道，不能删除", count)
	}
	return DB.Delete(&ChannelProvider{}, id).Error
}

func BuildChannelProviderTrees(channels []*Channel, offset int, limit int, idSort bool) ([]*ChannelProviderTree, int64) {
	AttachChannelProviderSummaries(channels)
	treeMap := make(map[int]*ChannelProviderTree)
	order := make([]int, 0)
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		providerID := channel.ProviderID
		if providerID == 0 {
			providerID = -channel.Id
		}
		tree, exists := treeMap[providerID]
		if !exists {
			name := "未归属供应商"
			baseURL := EffectiveBaseURLForChannel(channel)
			status := common.ChannelStatusManuallyDisabled
			if channel.Provider != nil {
				name = channel.Provider.Name
				baseURL = channel.Provider.BaseURL
				status = channel.Provider.Status
			}
			var providerSettings string
			var providerBalance float64
			var providerBalanceUpdatedTime int64
			if channel.Provider != nil {
				providerSettings = channel.Provider.Settings
				providerBalance = channel.Provider.Balance
				providerBalanceUpdatedTime = channel.Provider.BalanceUpdatedTime
			}
			tree = &ChannelProviderTree{
				Key:                fmt.Sprintf("provider-%d", providerID),
				Id:                 fmt.Sprintf("P%d", providerID),
				IsProvider:         true,
				ProviderID:         channel.ProviderID,
				Name:               name,
				BaseURL:            baseURL,
				Status:             status,
				Settings:           providerSettings,
				Balance:            providerBalance,
				BalanceUpdatedTime: providerBalanceUpdatedTime,
				Priority:           nil,
				Weight:             nil,
				Children:           make([]*Channel, 0),
			}
			treeMap[providerID] = tree
			order = append(order, providerID)
		}
		tree.Children = append(tree.Children, channel)
		tree.ChannelCount++
		tree.UsedQuota += channel.UsedQuota
		tree.ResponseTime += channel.ResponseTime
		if channel.Status == common.ChannelStatusEnabled {
			tree.EnabledCount++
			tree.Status = common.ChannelStatusEnabled
		}
		if tree.Group == "" {
			tree.Group = channel.Group
		} else {
			seenGroups := map[string]struct{}{}
			for _, group := range strings.Split(tree.Group, ",") {
				group = strings.TrimSpace(group)
				if group != "" {
					seenGroups[group] = struct{}{}
				}
			}
			for _, group := range strings.Split(channel.Group, ",") {
				group = strings.TrimSpace(group)
				if group == "" {
					continue
				}
				if _, ok := seenGroups[group]; !ok {
					tree.Group += "," + group
					seenGroups[group] = struct{}{}
				}
			}
		}
		if tree.Priority == nil {
			tree.Priority = channel.Priority
		} else if priority, ok := tree.Priority.(*int64); ok {
			if (priority == nil && channel.Priority != nil) ||
				(priority != nil && (channel.Priority == nil || *priority != *channel.Priority)) {
				tree.Priority = ""
			}
		}
		if tree.Weight == nil {
			tree.Weight = channel.Weight
		} else if weight, ok := tree.Weight.(*uint); ok {
			if (weight == nil && channel.Weight != nil) ||
				(weight != nil && (channel.Weight == nil || *weight != *channel.Weight)) {
				tree.Weight = ""
			}
		}
	}

	sort.SliceStable(order, func(i, j int) bool {
		left := treeMap[order[i]]
		right := treeMap[order[j]]
		leftEnabled := left.EnabledCount > 0
		rightEnabled := right.EnabledCount > 0
		if leftEnabled != rightEnabled {
			return leftEnabled
		}
		if idSort {
			return order[i] > order[j]
		}
		if left.EnabledCount != right.EnabledCount {
			return left.EnabledCount > right.EnabledCount
		}
		return left.ProviderID > right.ProviderID
	})

	total := int64(len(order))
	if offset < 0 {
		offset = 0
	}
	if offset > len(order) {
		offset = len(order)
	}
	end := len(order)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	trees := make([]*ChannelProviderTree, 0, end-offset)
	for _, providerID := range order[offset:end] {
		tree := treeMap[providerID]
		if tree.ChannelCount > 0 {
			tree.ResponseTime = tree.ResponseTime / tree.ChannelCount
		}
		trees = append(trees, tree)
	}
	return trees, total
}
