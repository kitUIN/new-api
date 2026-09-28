package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type RelayAssetSetting struct {
	CacheMB    int    `json:"cache_mb"`
	CacheDir   string `json:"cache_dir"`
	TTLSeconds int    `json:"ttl_seconds"`
}

var relayAssetSetting = RelayAssetSetting{8192, "data/relay-asset-cache", 3600}

func init()                                    { config.GlobalConfig.Register("relay_asset_setting", &relayAssetSetting) }
func GetRelayAssetSetting() *RelayAssetSetting { return &relayAssetSetting }
