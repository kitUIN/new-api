package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWalletSettlesActualUsageBeforeCheckingNextRequest(t *testing.T) {
	truncate(t)
	seedUser(t, 1, 10)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		UserId: 1, IsPlayground: true, ForcePreConsume: true,
		UserSetting: dto.UserSetting{BillingPreference: "wallet_only"},
	}
	session, apiErr := NewBillingSession(ctx, info, 10)
	require.Nil(t, apiErr)
	require.Zero(t, getUserQuota(t, 1))
	require.NoError(t, session.Settle(15))
	require.Equal(t, -5, getUserQuota(t, 1))

	_, apiErr = NewBillingSession(ctx, info, 1)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 1).Update("allow_negative_balance", true).Error)
	session, apiErr = NewBillingSession(ctx, info, 5)
	require.Nil(t, apiErr)
	require.Equal(t, -10, getUserQuota(t, 1))
	require.NoError(t, session.Settle(2))
	require.Equal(t, -7, getUserQuota(t, 1))
}
