package service

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// Check both the requested group and the concrete route, including affinity and
// explicit-channel routes which can bypass the usual channel selector.
func CheckRequestGroupOpeningHours(c *gin.Context) error {
	now := time.Now()
	for _, key := range []constant.ContextKey{
		constant.ContextKeyTokenGroup,
		constant.ContextKeyUsingGroup,
		constant.ContextKeyAutoGroup,
		constant.ContextKeyGroupCombination,
	} {
		if err := ratio_setting.CheckGroupOpenAt(common.GetContextKeyString(c, key), now); err != nil {
			return err
		}
	}
	return nil
}
