package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Ability struct {
	Group     string  `json:"group" gorm:"type:varchar(64);primaryKey;autoIncrement:false"`
	Model     string  `json:"model" gorm:"type:varchar(255);primaryKey;autoIncrement:false"`
	ChannelId int     `json:"channel_id" gorm:"primaryKey;autoIncrement:false;index"`
	Enabled   bool    `json:"enabled"`
	Priority  *int64  `json:"priority" gorm:"bigint;default:0;index"`
	Weight    uint    `json:"weight" gorm:"default:0;index"`
	Tag       *string `json:"tag" gorm:"index"`
}

type AbilityWithChannel struct {
	Ability
	ChannelType int `json:"channel_type"`
}

type EnabledGroupChannel struct {
	Group   string `json:"group"`
	Channel Channel
}

func GetAllEnableAbilityWithChannels() ([]AbilityWithChannel, error) {
	var abilities []AbilityWithChannel
	err := DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type").
		Joins("left join channels on abilities.channel_id = channels.id").
		Where("abilities.enabled = ?", true).
		Scan(&abilities).Error
	return abilities, err
}

func GetEnabledGroupChannels() ([]EnabledGroupChannel, error) {
	var rows []struct {
		Group     string
		ChannelId int
	}
	err := DB.Model(&Ability{}).
		Select(commonGroupCol+" as "+commonGroupCol+", channel_id").
		Where("enabled = ?", true).
		Group(commonGroupCol + ", channel_id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []EnabledGroupChannel{}, nil
	}

	channelIDs := make([]int, 0, len(rows))
	seenChannelIDs := make(map[int]struct{})
	for _, row := range rows {
		if row.ChannelId <= 0 {
			continue
		}
		if _, ok := seenChannelIDs[row.ChannelId]; ok {
			continue
		}
		seenChannelIDs[row.ChannelId] = struct{}{}
		channelIDs = append(channelIDs, row.ChannelId)
	}
	if len(channelIDs) == 0 {
		return []EnabledGroupChannel{}, nil
	}

	var channels []Channel
	if err := DB.Where("id IN ? AND status = ?", channelIDs, common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}
	channelByID := make(map[int]Channel, len(channels))
	for _, channel := range channels {
		channelByID[channel.Id] = channel
	}

	result := make([]EnabledGroupChannel, 0, len(rows))
	seenGroupChannels := make(map[string]struct{})
	for _, row := range rows {
		channel, ok := channelByID[row.ChannelId]
		if !ok {
			continue
		}
		key := row.Group + "|" + fmt.Sprintf("%d", row.ChannelId)
		if _, ok := seenGroupChannels[key]; ok {
			continue
		}
		seenGroupChannels[key] = struct{}{}
		result = append(result, EnabledGroupChannel{
			Group:   row.Group,
			Channel: channel,
		})
	}
	return result, nil
}

func GetEnabledChannelGroupSet() (map[string]bool, error) {
	var channels []Channel
	err := DB.Model(&Channel{}).
		Select(channelSatisfyGroupCol()).
		Where("status = ?", common.ChannelStatusEnabled).
		Find(&channels).Error
	if err != nil {
		return nil, err
	}

	groups := make(map[string]bool)
	for _, channel := range channels {
		for _, group := range strings.Split(channel.Group, ",") {
			group = strings.TrimSpace(group)
			if group != "" {
				groups[group] = true
			}
		}
	}
	return groups, nil
}

func GetGroupEnabledModels(group string) []string {
	var models []string
	// Find distinct models
	DB.Table("abilities").Where(channelSatisfyGroupCol()+" = ? and enabled = ?", group, true).Distinct("model").Pluck("model", &models)
	return models
}

func GetEnabledModels() []string {
	var models []string
	// Find distinct models
	DB.Table("abilities").Where("enabled = ?", true).Distinct("model").Pluck("model", &models)
	return models
}

func GetAllEnableAbilities() []Ability {
	var abilities []Ability
	DB.Find(&abilities, "enabled = ?", true)
	return abilities
}

func getPriority(group string, model string, retry int) (int, error) {
	return getPriorityWithExclusions(group, model, retry, nil)
}

func abilityGroupColumn() string {
	if commonGroupCol != "" {
		return "abilities." + commonGroupCol
	}
	if common.UsingPostgreSQL {
		return `abilities."group"`
	}
	return "abilities.`group`"
}

func getPriorityWithExclusions(group string, model string, retry int, excludedChannelIDs []int) (int, error) {

	var priorities []int
	groupCol := abilityGroupColumn()
	query := DB.Model(&Ability{}).
		Joins("JOIN channels ON channels.id = abilities.channel_id").
		Select("DISTINCT(abilities.priority)").
		Where(groupCol+" = ? and abilities.model = ? and abilities.enabled = ? and channels.status = ?", group, model, true, common.ChannelStatusEnabled)
	if len(excludedChannelIDs) > 0 {
		query = query.Where("abilities.channel_id NOT IN ?", excludedChannelIDs)
	}
	err := query.
		Order("abilities.priority DESC").    // 按优先级降序排序
		Pluck("priority", &priorities).Error // Pluck用于将查询的结果直接扫描到一个切片中

	if err != nil {
		// 处理错误
		return 0, err
	}

	if len(priorities) == 0 {
		// 如果没有查询到优先级，则返回错误
		return 0, errors.New("数据库一致性被破坏")
	}

	// 确定要使用的优先级
	var priorityToUse int
	if retry >= len(priorities) {
		// 如果重试次数大于优先级数，则使用最小的优先级
		priorityToUse = priorities[len(priorities)-1]
	} else {
		priorityToUse = priorities[retry]
	}
	return priorityToUse, nil
}

func getChannelQuery(group string, model string, retry int) (*gorm.DB, error) {
	return getChannelQueryWithExclusions(group, model, retry, nil)
}

func getChannelQueryWithExclusions(group string, model string, retry int, excludedChannelIDs []int) (*gorm.DB, error) {
	groupCol := abilityGroupColumn()
	maxPrioritySubQuery := DB.Model(&Ability{}).
		Joins("JOIN channels ON channels.id = abilities.channel_id").
		Select("MAX(abilities.priority)").
		Where(groupCol+" = ? and abilities.model = ? and abilities.enabled = ? and channels.status = ?", group, model, true, common.ChannelStatusEnabled)
	if len(excludedChannelIDs) > 0 {
		maxPrioritySubQuery = maxPrioritySubQuery.Where("abilities.channel_id NOT IN ?", excludedChannelIDs)
	}
	channelQuery := DB.Model(&Ability{}).
		Joins("JOIN channels ON channels.id = abilities.channel_id").
		Where(groupCol+" = ? and abilities.model = ? and abilities.enabled = ? and channels.status = ? and abilities.priority = (?)", group, model, true, common.ChannelStatusEnabled, maxPrioritySubQuery)
	if len(excludedChannelIDs) > 0 {
		channelQuery = channelQuery.Where("abilities.channel_id NOT IN ?", excludedChannelIDs)
	}
	if retry != 0 {
		priority, err := getPriorityWithExclusions(group, model, retry, excludedChannelIDs)
		if err != nil {
			return nil, err
		} else {
			channelQuery = DB.Model(&Ability{}).
				Joins("JOIN channels ON channels.id = abilities.channel_id").
				Where(groupCol+" = ? and abilities.model = ? and abilities.enabled = ? and channels.status = ? and abilities.priority = ?", group, model, true, common.ChannelStatusEnabled, priority)
			if len(excludedChannelIDs) > 0 {
				channelQuery = channelQuery.Where("abilities.channel_id NOT IN ?", excludedChannelIDs)
			}
		}
	}

	return channelQuery, nil
}

func GetChannel(group string, model string, retry int) (*Channel, error) {
	return GetChannelWithExclusions(group, model, retry, nil)
}

func GetChannelWithExclusions(group string, model string, retry int, excludedChannelIDs []int) (*Channel, error) {
	var abilities []Ability

	var err error = nil
	channelQuery, err := getChannelQueryWithExclusions(group, model, retry, excludedChannelIDs)
	if err != nil {
		return nil, err
	}
	if common.UsingSQLite || common.UsingPostgreSQL {
		err = channelQuery.Order("abilities.weight DESC").Find(&abilities).Error
	} else {
		err = channelQuery.Order("abilities.weight DESC").Find(&abilities).Error
	}
	if err != nil {
		return nil, err
	}
	channel := Channel{}
	if len(abilities) > 0 {
		// Randomly choose one
		weightSum := uint(0)
		for _, ability_ := range abilities {
			weightSum += ability_.Weight + 10
		}
		// Randomly choose one
		weight := common.GetRandomInt(int(weightSum))
		for _, ability_ := range abilities {
			weight -= int(ability_.Weight) + 10
			//log.Printf("weight: %d, ability weight: %d", weight, *ability_.Weight)
			if weight <= 0 {
				channel.Id = ability_.ChannelId
				break
			}
		}
	} else {
		return nil, nil
	}
	err = DB.First(&channel, "id = ?", channel.Id).Error
	return &channel, err
}

func (channel *Channel) AddAbilities(tx *gorm.DB) error {
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}
	if len(abilities) == 0 {
		return nil
	}
	// choose DB or provided tx
	useDB := DB
	if tx != nil {
		useDB = tx
	}
	for _, chunk := range lo.Chunk(abilities, 50) {
		err := useDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func (channel *Channel) DeleteAbilities() error {
	return DB.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
}

// UpdateAbilities updates abilities of this channel.
// Make sure the channel is completed before calling this function.
func (channel *Channel) UpdateAbilities(tx *gorm.DB) error {
	isNewTx := false
	// 如果没有传入事务，创建新的事务
	if tx == nil {
		tx = DB.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		isNewTx = true
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()
	}

	// First delete all abilities of this channel
	err := tx.Where("channel_id = ?", channel.Id).Delete(&Ability{}).Error
	if err != nil {
		if isNewTx {
			tx.Rollback()
		}
		return err
	}

	// Then add new abilities
	models_ := strings.Split(channel.Models, ",")
	groups_ := strings.Split(channel.Group, ",")
	abilitySet := make(map[string]struct{})
	abilities := make([]Ability, 0, len(models_))
	for _, model := range models_ {
		for _, group := range groups_ {
			key := group + "|" + model
			if _, exists := abilitySet[key]; exists {
				continue
			}
			abilitySet[key] = struct{}{}
			ability := Ability{
				Group:     group,
				Model:     model,
				ChannelId: channel.Id,
				Enabled:   channel.Status == common.ChannelStatusEnabled,
				Priority:  channel.Priority,
				Weight:    uint(channel.GetWeight()),
				Tag:       channel.Tag,
			}
			abilities = append(abilities, ability)
		}
	}

	if len(abilities) > 0 {
		for _, chunk := range lo.Chunk(abilities, 50) {
			err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
			if err != nil {
				if isNewTx {
					tx.Rollback()
				}
				return err
			}
		}
	}

	// 如果是新创建的事务，需要提交
	if isNewTx {
		return tx.Commit().Error
	}

	return nil
}

func UpdateAbilityStatus(channelId int, status bool) error {
	return DB.Model(&Ability{}).Where("channel_id = ?", channelId).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityStatusByTag(tag string, status bool) error {
	return DB.Model(&Ability{}).Where("tag = ?", tag).Select("enabled").Update("enabled", status).Error
}

func UpdateAbilityByTag(tag string, newTag *string, priority *int64, weight *uint) error {
	ability := Ability{}
	if newTag != nil {
		ability.Tag = newTag
	}
	if priority != nil {
		ability.Priority = priority
	}
	if weight != nil {
		ability.Weight = *weight
	}
	return DB.Model(&Ability{}).Where("tag = ?", tag).Updates(ability).Error
}

var fixLock = sync.Mutex{}

func FixAbility() (int, int, error) {
	lock := fixLock.TryLock()
	if !lock {
		return 0, 0, errors.New("已经有一个修复任务在运行中，请稍后再试")
	}
	defer fixLock.Unlock()

	// truncate abilities table
	if common.UsingSQLite {
		err := DB.Exec("DELETE FROM abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	} else {
		err := DB.Exec("TRUNCATE TABLE abilities").Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Truncate abilities failed: %s", err.Error()))
			return 0, 0, err
		}
	}
	var channels []*Channel
	// Find all channels
	err := DB.Model(&Channel{}).Find(&channels).Error
	if err != nil {
		return 0, 0, err
	}
	if len(channels) == 0 {
		return 0, 0, nil
	}
	successCount := 0
	failCount := 0
	for _, chunk := range lo.Chunk(channels, 50) {
		ids := lo.Map(chunk, func(c *Channel, _ int) int { return c.Id })
		// Delete all abilities of this channel
		err = DB.Where("channel_id IN ?", ids).Delete(&Ability{}).Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Delete abilities failed: %s", err.Error()))
			failCount += len(chunk)
			continue
		}
		// Then add new abilities
		for _, channel := range chunk {
			err = channel.AddAbilities(nil)
			if err != nil {
				common.SysLog(fmt.Sprintf("Add abilities for channel %d failed: %s", channel.Id, err.Error()))
				failCount++
			} else {
				successCount++
			}
		}
	}
	InitChannelCache()
	return successCount, failCount, nil
}
