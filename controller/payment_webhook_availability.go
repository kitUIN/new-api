package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

func isStripeTopUpEnabled() bool {
	return strings.TrimSpace(setting.StripeApiSecret) != "" &&
		strings.TrimSpace(setting.StripeWebhookSecret) != "" &&
		strings.TrimSpace(setting.StripePriceId) != ""
}

func isStripeWebhookConfigured() bool {
	return strings.TrimSpace(setting.StripeWebhookSecret) != ""
}

func isStripeWebhookEnabled() bool {
	return isStripeTopUpEnabled()
}

func isCreemTopUpEnabled() bool {
	products := strings.TrimSpace(setting.CreemProducts)
	return strings.TrimSpace(setting.CreemApiKey) != "" &&
		products != "" &&
		products != "[]"
}

func isCreemWebhookConfigured() bool {
	return strings.TrimSpace(setting.CreemWebhookSecret) != ""
}

func isCreemWebhookEnabled() bool {
	return isCreemTopUpEnabled() && isCreemWebhookConfigured()
}

func isWaffoTopUpEnabled() bool {
	if !setting.WaffoEnabled {
		return false
	}

	return isWaffoWebhookConfigured()
}

func isWaffoWebhookConfigured() bool {
	if setting.WaffoSandbox {
		return strings.TrimSpace(setting.WaffoSandboxApiKey) != "" &&
			strings.TrimSpace(setting.WaffoSandboxPrivateKey) != "" &&
			strings.TrimSpace(setting.WaffoSandboxPublicCert) != ""
	}

	return strings.TrimSpace(setting.WaffoApiKey) != "" &&
		strings.TrimSpace(setting.WaffoPrivateKey) != "" &&
		strings.TrimSpace(setting.WaffoPublicCert) != ""
}

func isWaffoWebhookEnabled() bool {
	return isWaffoTopUpEnabled()
}

func isWaffoPancakeTopUpEnabled() bool {
	if !setting.WaffoPancakeEnabled {
		return false
	}

	return isWaffoPancakeWebhookConfigured() &&
		strings.TrimSpace(setting.WaffoPancakeMerchantID) != "" &&
		strings.TrimSpace(setting.WaffoPancakePrivateKey) != "" &&
		strings.TrimSpace(setting.WaffoPancakeStoreID) != "" &&
		strings.TrimSpace(setting.WaffoPancakeProductID) != ""
}

func isWaffoPancakeWebhookConfigured() bool {
	currentWebhookKey := strings.TrimSpace(setting.WaffoPancakeWebhookPublicKey)
	if setting.WaffoPancakeSandbox {
		currentWebhookKey = strings.TrimSpace(setting.WaffoPancakeWebhookTestKey)
	}

	return currentWebhookKey != ""
}

func isWaffoPancakeWebhookEnabled() bool {
	return isWaffoPancakeTopUpEnabled()
}

func isEpayTopUpEnabled() bool {
	return isEpayWebhookConfigured() && len(operation_setting.PayMethods) > 0
}

func isEpayWebhookConfigured() bool {
	return strings.TrimSpace(operation_setting.PayAddress) != "" &&
		strings.TrimSpace(operation_setting.EpayId) != "" &&
		strings.TrimSpace(operation_setting.EpayKey) != ""
}

func isEpayWebhookEnabled() bool {
	return isEpayTopUpEnabled()
}

func isXznPayTopUpEnabled() bool {
	if !setting.XznPayEnabled || len(setting.GetXznPayMethods()) == 0 {
		return false
	}
	if strings.TrimSpace(setting.XznPayGatewayURL) == "" || strings.TrimSpace(setting.XznPayPID) == "" {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(setting.XznPaySignType)) {
	case setting.XznPaySignTypeMD5:
		return strings.TrimSpace(setting.XznPayMD5Key) != ""
	case setting.XznPaySignTypeRSA:
		return strings.TrimSpace(setting.XznPayPrivateKey) != "" && strings.TrimSpace(setting.XznPayPublicKey) != ""
	default:
		return false
	}
}

func isXznPayWebhookEnabled() bool {
	return isXznPayTopUpEnabled()
}
