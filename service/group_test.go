package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCombinationGroupUsesPricingDescription(t *testing.T) {
	oldDB := model.DB
	oldRatios := ratio_setting.GroupRatio2JSONString()
	oldCombinations := ratio_setting.GroupCombinations2JSONString()
	oldUsable := setting.UserUsableGroups2JSONString()
	special := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
	oldSpecial := special.ReadAll()
	t.Cleanup(func() {
		model.DB = oldDB
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios))
		require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(oldCombinations))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldUsable))
		special.Clear()
		special.AddAll(oldSpecial)
	})
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	require.NoError(t, db.Create(&model.Channel{Key: "test", Group: "source", Status: common.ChannelStatusEnabled}).Error)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"combo":1,"source":1,"second":2}`))
	require.NoError(t, ratio_setting.UpdateGroupCombinationsByJSONString(`{"combo":[{"group":"source","models":["test-model"]},{"group":"second","models":["test-model"]}]}`))
	special.Clear()
	special.AddAll(map[string]map[string]string{"user": {"+:combo": "combo"}})
	for _, configured := range []string{
		`{"combo":"Pricing description"}`,
		`{"__disabled_description__:combo":"Pricing description"}`,
	} {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(configured))
		groups, err := GetSortedUserUsableGroupInfos("user")
		require.NoError(t, err)
		found := false
		for _, group := range groups {
			if group.Name == "combo" {
				found = true
				require.True(t, group.IsCombination)
				require.Equal(t, "Pricing description", group.Desc)
				require.Equal(t, "Pricing description", UserUsableGroupInfosToMap(groups)["combo"]["desc"])
			}
		}
		require.True(t, found)
	}
}
