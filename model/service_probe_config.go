package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// The Secret suffix also excludes the complete document from generic option reads.
const ServiceProbeConfigOption = "ServiceProbeConfigSecret"

type ServiceProbeConfig struct {
	Enabled    bool   `json:"enabled"`
	URL        string `json:"url"`
	ReadAPIKey string `json:"read_api_key"`
}

func GetServiceProbeConfig() (ServiceProbeConfig, bool, error) {
	common.OptionMapRWMutex.RLock()
	raw, exists := common.OptionMap[ServiceProbeConfigOption]
	common.OptionMapRWMutex.RUnlock()
	var config ServiceProbeConfig
	if !exists {
		return config, false, nil
	}
	err := common.UnmarshalJsonStr(raw, &config)
	return config, true, err
}

func SaveServiceProbeConfig(config ServiceProbeConfig) error {
	data, err := common.Marshal(config)
	if err != nil {
		return err
	}
	option := Option{Key: ServiceProbeConfigOption, Value: string(data)}
	// Publish the entire configuration only after its single-row write succeeds.
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	// SQL logging must not expose the stored read key, including failed writes.
	db := DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&option).Error; err != nil {
		return err
	}
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[ServiceProbeConfigOption] = option.Value
	return nil
}
