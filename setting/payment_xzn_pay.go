package setting

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	XznPayDefaultGateway = "https://pay.xzncraft.cn"
	XznPaySignTypeMD5    = "MD5"
	XznPaySignTypeRSA    = "RSA"
)

var (
	XznPayEnabled         bool
	XznPayGatewayURL      = XznPayDefaultGateway
	XznPayCallbackAddress string
	XznPayPID             string
	XznPaySignType        = XznPaySignTypeMD5
	XznPayMD5Key          string
	XznPayPrivateKey      string
	XznPayPublicKey       string
	XznPayMinTopUp        = 1
)

type XznPayMethod struct {
	Name        string `json:"name"`
	PayTypeCode string `json:"paytype_code"`
	ChannelID   string `json:"channel_id,omitempty"`
	Icon        string `json:"icon,omitempty"`
	MinTopUp    int    `json:"min_topup,omitempty"`
}

func GetXznPayMethods() []XznPayMethod {
	common.OptionMapRWMutex.RLock()
	jsonString := common.OptionMap["XznPayMethods"]
	common.OptionMapRWMutex.RUnlock()

	if strings.TrimSpace(jsonString) == "" {
		return []XznPayMethod{}
	}

	var methods []XznPayMethod
	if err := common.UnmarshalJsonStr(jsonString, &methods); err != nil {
		return []XznPayMethod{}
	}
	return methods
}

func ValidateXznPayMethodsJson(jsonString string) error {
	var methods []XznPayMethod
	if err := common.UnmarshalJsonStr(jsonString, &methods); err != nil {
		return err
	}
	for index, method := range methods {
		if strings.TrimSpace(method.Name) == "" {
			return fmt.Errorf("XznPay 支付方式 %d 缺少名称", index+1)
		}
		if strings.TrimSpace(method.PayTypeCode) == "" {
			return fmt.Errorf("XznPay 支付方式 %d 缺少 paytype_code", index+1)
		}
		if method.MinTopUp < 0 {
			return fmt.Errorf("XznPay 支付方式 %d 的最低充值金额不能为负数", index+1)
		}
	}
	return nil
}

func XznPayMethods2JsonString() string {
	return "[]"
}
