package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDrawingModelsTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Ability{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	return db
}

func TestGetDrawingModels(t *testing.T) {
	db := setupDrawingModelsTest(t)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: drawingGroup, Model: "image-z", ChannelId: 1, Enabled: true},
		{Group: drawingGroup, Model: "image-a", ChannelId: 2, Enabled: true},
		{Group: drawingGroup, Model: "image-a", ChannelId: 3, Enabled: true},
		{Group: drawingGroup, Model: "disabled", ChannelId: 4, Enabled: false},
		{Group: "other", Model: "other-model", ChannelId: 5, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	GetDrawingModels(ctx)
	var response struct {
		Success bool     `json:"success"`
		Data    []string `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, []string{"image-a", "image-z"}, response.Data)

	require.NoError(t, db.Where("1 = 1").Delete(&model.Ability{}).Error)
	recorder = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(recorder)
	GetDrawingModels(ctx)
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.NotNil(t, response.Data)
	require.Empty(t, response.Data)
}

func TestSubmitDrawingTaskRejectsUnavailableModels(t *testing.T) {
	db := setupDrawingModelsTest(t)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: drawingGroup, Model: "disabled", ChannelId: 1, Enabled: false},
		{Group: "other", Model: "other-model", ChannelId: 2, Enabled: true},
	}).Error)
	for _, name := range []string{"disabled", "other-model", "missing"} {
		t.Run(name, func(t *testing.T) {
			body, err := common.Marshal(DrawingGenerateRequest{Prompt: "test", Model: name})
			require.NoError(t, err)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body)))
			ctx.Request.Header.Set("Content-Type", "application/json")
			SubmitDrawingTask(ctx)
			var response struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.False(t, response.Success)
			require.Equal(t, "所选模型不在绘图分组中或已停用", response.Message)
		})
	}
}
