package controller

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type modelListResponse struct {
	Data []dto.OpenAIModels `json:"data"`
}

func setupModelGroupCombinationControllerTest(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := model.DB
	originalSelfUseMode := operation_setting.SelfUseModeEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		model.DB = originalDB
		operation_setting.SelfUseModeEnabled = originalSelfUseMode
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
	})

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	operation_setting.SelfUseModeEnabled = true
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	require.NoError(t, db.Create([]model.Ability{
		{Group: "group-a", Model: "gpt-5.6", ChannelId: 1, Enabled: true},
		{Group: "group-a", Model: "shared-model", ChannelId: 1, Enabled: true},
		{Group: "group-b", Model: "deepseek-v4-flash", ChannelId: 2, Enabled: true},
		{Group: "group-b", Model: "shared-model", ChannelId: 2, Enabled: true},
	}).Error)
	return db
}

func newModelCombinationListContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelGroupCombinationGroups, `["group-a","group-b"]`)
	return ctx, recorder
}

func decodeModelIDs(t *testing.T, recorder *httptest.ResponseRecorder) []string {
	t.Helper()
	var response modelListResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	ids := make([]string, 0, len(response.Data))
	for _, item := range response.Data {
		ids = append(ids, item.Id)
	}
	return ids
}

func TestListModelsReturnsCombinationUnion(t *testing.T) {
	setupModelGroupCombinationControllerTest(t)
	ctx, recorder := newModelCombinationListContext(t)

	ListModels(ctx, constant.ChannelTypeOpenAI)

	require.ElementsMatch(t, []string{"gpt-5.6", "deepseek-v4-flash", "shared-model"}, decodeModelIDs(t, recorder))
}

func TestListModelsIntersectsCombinationWithTokenModelLimits(t *testing.T) {
	setupModelGroupCombinationControllerTest(t)
	ctx, recorder := newModelCombinationListContext(t)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(ctx, constant.ContextKeyTokenModelLimit, map[string]bool{
		"deepseek-v4-flash": true,
	})

	ListModels(ctx, constant.ChannelTypeOpenAI)

	require.Equal(t, []string{"deepseek-v4-flash"}, decodeModelIDs(t, recorder))
}
