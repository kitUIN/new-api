package model

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
)

func parseRequestDetailLogOption(key, value string) (int32, error) {
	if key == "LogRequestDetailEnabled" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return common.RequestDetailLogOff, err
		}
		if enabled {
			return common.RequestDetailLogFailed, nil
		}
		return common.RequestDetailLogOff, nil
	}
	return common.ParseRequestDetailLogMode(value)
}
