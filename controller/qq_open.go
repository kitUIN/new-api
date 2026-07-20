package controller

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type qqOpenTokenResponse struct {
	Id                 int    `json:"id"`
	Name               string `json:"name"`
	Key                string `json:"key"`
	Status             int    `json:"status"`
	CreatedTime        int64  `json:"created_time"`
	AccessedTime       int64  `json:"accessed_time"`
	ExpiredTime        int64  `json:"expired_time"`
	RemainQuota        int    `json:"remain_quota"`
	UsedQuota          int    `json:"used_quota"`
	UnlimitedQuota     bool   `json:"unlimited_quota"`
	ModelLimitsEnabled bool   `json:"model_limits_enabled"`
	ModelLimits        string `json:"model_limits"`
	Group              string `json:"group"`
	CrossGroupRetry    bool   `json:"cross_group_retry"`
	AutoGroupMode      string `json:"auto_group_mode"`
}

type qqOpenUpdateTokenGroupRequest struct {
	Group string `json:"group"`
}

func qqOpenUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"message": "unauthorized",
	})
}

func requireQQOpenAccess(c *gin.Context) bool {
	expectedToken := strings.TrimSpace(common.QQCallbackAccessToken)
	if expectedToken == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未配置 QQ 服务 accessToken",
		})
		return false
	}
	token := strings.TrimSpace(c.GetHeader("X-Access-Token"))
	if token == "" {
		token = strings.TrimSpace(c.GetHeader("Authorization"))
	}
	token = strings.TrimPrefix(token, "Bearer ")
	if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		qqOpenUnauthorized(c)
		return false
	}
	return true
}

func getQQOpenUser(c *gin.Context) (*model.User, bool) {
	if !requireQQOpenAccess(c) {
		return nil, false
	}
	qqId := strings.TrimSpace(c.Param("qq_id"))
	if qqId == "" {
		common.ApiError(c, errors.New("QQ 号为空"))
		return nil, false
	}
	user := &model.User{QQId: qqId}
	if err := user.FillUserByQQId(); err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	return user, true
}

func buildQQOpenTokenResponse(token *model.Token) qqOpenTokenResponse {
	return qqOpenTokenResponse{
		Id:                 token.Id,
		Name:               token.Name,
		Key:                token.GetMaskedKey(),
		Status:             token.Status,
		CreatedTime:        token.CreatedTime,
		AccessedTime:       token.AccessedTime,
		ExpiredTime:        token.ExpiredTime,
		RemainQuota:        token.RemainQuota,
		UsedQuota:          token.UsedQuota,
		UnlimitedQuota:     token.UnlimitedQuota,
		ModelLimitsEnabled: token.ModelLimitsEnabled,
		ModelLimits:        token.ModelLimits,
		Group:              token.Group,
		CrossGroupRetry:    token.CrossGroupRetry,
		AutoGroupMode:      token.AutoGroupMode,
	}
}

func buildQQOpenTokenResponses(tokens []*model.Token) []qqOpenTokenResponse {
	responses := make([]qqOpenTokenResponse, 0, len(tokens))
	for _, token := range tokens {
		responses = append(responses, buildQQOpenTokenResponse(token))
	}
	return responses
}

func GetQQUserTokens(c *gin.Context) {
	user, ok := getQQOpenUser(c)
	if !ok {
		return
	}
	tokens, err := model.GetTokensByUserId(user.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"qq_id":   user.QQId,
		"user_id": user.Id,
		"tokens":  buildQQOpenTokenResponses(tokens),
	})
}

func GetQQUserGroups(c *gin.Context) {
	user, ok := getQQOpenUser(c)
	if !ok {
		return
	}
	groups, err := service.GetSortedUserUsableGroupInfos(user.Group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"qq_id":         user.QQId,
		"user_id":       user.Id,
		"user_group":    user.Group,
		"usable_groups": service.UserUsableGroupInfosToMap(groups),
		"groups":        groups,
	})
}

func GetQQGroupHealthSummary(c *gin.Context) {
	if !requireQQOpenAccess(c) {
		return
	}
	hours := getPerfMetricsHours(c)
	intervalMinutes := getPerfMetricsGroupIntervalMinutes(c)
	summary, err := model.GetPerfGroupHealthSummary(hours, intervalMinutes)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func UpdateQQUserTokenGroup(c *gin.Context) {
	user, ok := getQQOpenUser(c)
	if !ok {
		return
	}
	var req qqOpenUpdateTokenGroupRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	group := strings.TrimSpace(req.Group)
	if group != "" && !service.GroupInUserUsableGroups(user.Group, group) {
		common.ApiError(c, errors.New("无权访问该分组"))
		return
	}
	tokenId, err := parseTokenIdParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.UpdateTokenGroupByIds(tokenId, user.Id, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"qq_id":   user.QQId,
		"user_id": user.Id,
		"token":   buildQQOpenTokenResponse(token),
	})
}

func parseTokenIdParam(c *gin.Context) (int, error) {
	tokenId := common.String2Int(c.Param("token_id"))
	if tokenId <= 0 {
		return 0, errors.New("token_id 无效")
	}
	return tokenId, nil
}
