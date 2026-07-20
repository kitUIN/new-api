package service

import (
	"context"
	"crypto"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	xznPayCreateOrderPath  = "/openapi/pay/create"
	xznPayMaxResponseBytes = 1024 * 1024
	xznPayRequestTimeout   = 15 * time.Second
)

type XznPayConfig struct {
	GatewayURL string
	PID        string
	SignType   string
	MD5Key     string
	PrivateKey string
	PublicKey  string
}

type XznPayClient struct {
	config     XznPayConfig
	httpClient *http.Client
}

type XznPayCreateOrderData struct {
	TradeNo    string `json:"trade_no"`
	OutTradeNo string `json:"out_trade_no"`
	PayURL     string `json:"pay_url"`
}

type XznPayCreateOrderResponse struct {
	Code int                   `json:"code"`
	Msg  string                `json:"msg"`
	Data XznPayCreateOrderData `json:"data"`
}

func NewXznPayClient(config XznPayConfig, httpClient *http.Client) (*XznPayClient, error) {
	config.GatewayURL = strings.TrimRight(strings.TrimSpace(config.GatewayURL), "/")
	config.PID = strings.TrimSpace(config.PID)
	config.SignType = strings.ToUpper(strings.TrimSpace(config.SignType))
	if config.GatewayURL == "" || config.PID == "" {
		return nil, errors.New("XznPay 网关地址和 PID 不能为空")
	}
	parsedURL, err := url.Parse(config.GatewayURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, errors.New("XznPay 网关地址无效")
	}
	switch config.SignType {
	case "MD5":
		if strings.TrimSpace(config.MD5Key) == "" {
			return nil, errors.New("XznPay MD5 密钥不能为空")
		}
	case "RSA":
		if strings.TrimSpace(config.PrivateKey) == "" || strings.TrimSpace(config.PublicKey) == "" {
			return nil, errors.New("XznPay RSA 私钥和平台公钥不能为空")
		}
	default:
		return nil, errors.New("XznPay 签名类型必须为 MD5 或 RSA")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: xznPayRequestTimeout}
	}
	return &XznPayClient{config: config, httpClient: httpClient}, nil
}

func (client *XznPayClient) CreateOrder(ctx context.Context, params map[string]string) (*XznPayCreateOrderResponse, error) {
	requestParams := copyXznPayParams(params)
	requestParams["pid"] = client.config.PID
	requestParams["timestamp"] = fmt.Sprintf("%d", time.Now().Unix())
	requestParams["sign_type"] = client.config.SignType
	signature, err := client.sign(requestParams)
	if err != nil {
		return nil, err
	}
	requestParams["sign"] = signature

	form := url.Values{}
	for key, value := range requestParams {
		form.Set(key, value)
	}
	requestContext, cancel := context.WithTimeout(ctx, xznPayRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestContext, http.MethodPost, client.config.GatewayURL+xznPayCreateOrderPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, xznPayMaxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > xznPayMaxResponseBytes {
		return nil, errors.New("XznPay 响应过大")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("XznPay HTTP 状态异常: %d", resp.StatusCode)
	}

	var result XznPayCreateOrderResponse
	if err := common.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 XznPay 响应失败: %w", err)
	}
	if result.Code != 1 {
		return nil, fmt.Errorf("XznPay 下单失败: %s", strings.TrimSpace(result.Msg))
	}
	if strings.TrimSpace(result.Data.TradeNo) == "" || strings.TrimSpace(result.Data.OutTradeNo) == "" {
		return nil, errors.New("XznPay 响应缺少订单号")
	}
	if result.Data.OutTradeNo != requestParams["out_trade_no"] {
		return nil, errors.New("XznPay 响应商户订单号不匹配")
	}
	if strings.TrimSpace(result.Data.PayURL) == "" {
		return nil, errors.New("XznPay 响应缺少 pay_url")
	}
	payURL, err := url.Parse(strings.TrimSpace(result.Data.PayURL))
	if err != nil || payURL.Host == "" || (payURL.Scheme != "http" && payURL.Scheme != "https") {
		return nil, errors.New("XznPay 响应中的 pay_url 无效")
	}
	return &result, nil
}

func (client *XznPayClient) VerifyNotify(params map[string]string) bool {
	signature := strings.TrimSpace(params["sign"])
	signType := strings.ToUpper(strings.TrimSpace(params["sign_type"]))
	if signType == "" {
		signType = "MD5"
	}
	if signature == "" || signType != client.config.SignType {
		return false
	}
	signString := buildXznPaySignString(params)
	if signType == "RSA" {
		publicKey, err := parseXznPayPublicKey(client.config.PublicKey)
		if err != nil {
			return false
		}
		signatureBytes, err := base64.StdEncoding.DecodeString(signature)
		if err != nil {
			return false
		}
		hash := sha256.Sum256([]byte(signString))
		return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], signatureBytes) == nil
	}
	expected := strings.ToUpper(fmt.Sprintf("%x", md5.Sum([]byte(signString+"&key="+client.config.MD5Key))))
	actual := strings.ToUpper(signature)
	return len(actual) == len(expected) && subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func (client *XznPayClient) sign(params map[string]string) (string, error) {
	signString := buildXznPaySignString(params)
	if client.config.SignType == "RSA" {
		privateKey, err := parseXznPayPrivateKey(client.config.PrivateKey)
		if err != nil {
			return "", err
		}
		hash := sha256.Sum256([]byte(signString))
		signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hash[:])
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(signature), nil
	}
	return strings.ToUpper(fmt.Sprintf("%x", md5.Sum([]byte(signString+"&key="+client.config.MD5Key)))), nil
}

func buildXznPaySignString(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key == "sign" || key == "sign_type" || value == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	return strings.Join(parts, "&")
}

func copyXznPayParams(params map[string]string) map[string]string {
	copyParams := make(map[string]string, len(params))
	for key, value := range params {
		copyParams[key] = value
	}
	return copyParams
}

func parseXznPayPrivateKey(value string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(normalizeXznPayPEM(value, "PRIVATE KEY")))
	if block == nil {
		return nil, errors.New("XznPay RSA 私钥解析失败")
	}
	if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if privateKey, ok := parsed.(*rsa.PrivateKey); ok {
			return privateKey, nil
		}
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func parseXznPayPublicKey(value string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(normalizeXznPayPEM(value, "PUBLIC KEY")))
	if block == nil {
		return nil, errors.New("XznPay RSA 公钥解析失败")
	}
	if certificate, err := x509.ParseCertificate(block.Bytes); err == nil {
		if publicKey, ok := certificate.PublicKey.(*rsa.PublicKey); ok {
			return publicKey, nil
		}
	}
	if parsed, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if publicKey, ok := parsed.(*rsa.PublicKey); ok {
			return publicKey, nil
		}
	}
	return x509.ParsePKCS1PublicKey(block.Bytes)
}

func normalizeXznPayPEM(value string, keyType string) string {
	trimmed := strings.TrimSpace(value)
	if strings.Contains(trimmed, "-----BEGIN") {
		return trimmed
	}
	compact := strings.NewReplacer("\r", "", "\n", "", " ", "").Replace(trimmed)
	chunks := make([]string, 0, (len(compact)+63)/64)
	for len(compact) > 64 {
		chunks = append(chunks, compact[:64])
		compact = compact[64:]
	}
	if compact != "" {
		chunks = append(chunks, compact)
	}
	return "-----BEGIN " + keyType + "-----\n" + strings.Join(chunks, "\n") + "\n-----END " + keyType + "-----"
}
