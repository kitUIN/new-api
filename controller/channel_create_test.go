package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAddChannelLeavesGroupsUnassigned(t *testing.T) {
	for _, mode := range []string{"single", "batch", "multi_to_single"} {
		for _, legacyGroup := range []bool{false, true} {
			name := mode + "/omitted_group"
			if legacyGroup {
				name = mode + "/legacy_group"
			}
			t.Run(name, func(t *testing.T) {
				db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
				require.NoError(t, err)
				sqlDB, err := db.DB()
				require.NoError(t, err)
				sqlDB.SetMaxOpenConns(1)
				originalDB := model.DB
				model.DB = db
				t.Cleanup(func() {
					model.DB = originalDB
					require.NoError(t, sqlDB.Close())
				})
				require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.ChannelProvider{}, &model.Ability{}))
				provider := model.ChannelProvider{Name: "provider", BaseURL: "http://provider.example"}
				require.NoError(t, db.Create(&provider).Error)
				key := "test-key-one"
				if mode != "single" {
					key += "\ntest-key-two"
				}
				channel := map[string]any{
					"provider_id": provider.Id,
					"type":        14,
					"name":        "unassigned",
					"key":         key,
					"models":      "claude-sonnet-5-5,claude-haiku-5-5,claude-opus-5-5",
					"status":      common.ChannelStatusEnabled,
				}
				if legacyGroup {
					channel["group"] = "codex-pro-正价菲区,codex-pro-正价luna,grok-正价,codex-pro正价,codex-pro-正价,codex-plus-正价luna"
				}
				body, err := common.Marshal(map[string]any{
					"mode": mode, "multi_key_mode": "random", "channel": channel,
				})
				require.NoError(t, err)
				router := gin.New()
				router.POST("/api/channel/", AddChannel)
				request := httptest.NewRequest(http.MethodPost, "/api/channel/", bytes.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				require.Equal(t, http.StatusOK, recorder.Code)
				var response struct {
					Success bool   `json:"success"`
					Message string `json:"message"`
				}
				require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
				require.True(t, response.Success, response.Message)

				var stored []model.Channel
				require.NoError(t, db.Find(&stored).Error)
				count := 1
				if mode == "batch" {
					count = 2
				}
				require.Len(t, stored, count)
				for _, created := range stored {
					require.Empty(t, created.Group)
					require.Equal(t, provider.Id, created.ProviderID)
					require.Equal(t, channel["models"], created.Models)
					require.Equal(t, mode == "multi_to_single", created.ChannelInfo.IsMultiKey)
				}
				var abilityCount int64
				require.NoError(t, db.Model(&model.Ability{}).Count(&abilityCount).Error)
				require.Zero(t, abilityCount, "new channels must not be routable until assigned from the group page")
			})
		}
	}
}
