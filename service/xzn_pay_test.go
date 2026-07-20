package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXznPayMD5SignAndVerify(t *testing.T) {
	client, err := NewXznPayClient(XznPayConfig{
		GatewayURL: "https://pay.example.com",
		PID:        "20880001",
		SignType:   "MD5",
		MD5Key:     "secret",
	}, nil)
	require.NoError(t, err)

	params := map[string]string{
		"pid":          "20880001",
		"out_trade_no": "ORDER1",
		"total_amount": "10.00",
		"empty":        "",
		"sign_type":    "MD5",
	}
	signature, err := client.sign(params)
	require.NoError(t, err)
	params["sign"] = signature
	require.True(t, client.VerifyNotify(params))

	delete(params, "sign_type")
	require.True(t, client.VerifyNotify(params))
	params["sign_type"] = "MD5"

	params["total_amount"] = "11.00"
	require.False(t, client.VerifyNotify(params))
	params["sign_type"] = "RSA"
	require.False(t, client.VerifyNotify(params))
}

func TestXznPayRSASignAndVerify(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	client, err := NewXznPayClient(XznPayConfig{
		GatewayURL: "https://pay.example.com",
		PID:        "20880001",
		SignType:   "RSA",
		PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})),
		PublicKey:  string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})),
	}, nil)
	require.NoError(t, err)

	params := map[string]string{
		"pid":          "20880001",
		"out_trade_no": "ORDER1",
		"total_amount": "10.00",
		"sign_type":    "RSA",
	}
	signature, err := client.sign(params)
	require.NoError(t, err)
	params["sign"] = signature
	require.True(t, client.VerifyNotify(params))

	params["out_trade_no"] = "ORDER2"
	require.False(t, client.VerifyNotify(params))
}

func TestXznPayCreateOrder(t *testing.T) {
	var receivedPath string
	var receivedForm map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		require.NoError(t, r.ParseForm())
		receivedForm = map[string]string{}
		for key := range r.PostForm {
			receivedForm[key] = r.PostForm.Get(key)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"code":1,"msg":"success","data":{"trade_no":"P1","out_trade_no":"ORDER1","pay_url":"https://cashier.example.com/pay"}}`)
	}))
	defer server.Close()

	client, err := NewXznPayClient(XznPayConfig{
		GatewayURL: server.URL,
		PID:        "20880001",
		SignType:   "MD5",
		MD5Key:     "secret",
	}, server.Client())
	require.NoError(t, err)
	response, err := client.CreateOrder(context.Background(), map[string]string{
		"out_trade_no": "ORDER1",
		"total_amount": "10.00",
		"subject":      "test",
		"paytype_code": "alipay",
	})
	require.NoError(t, err)
	assert.Equal(t, xznPayCreateOrderPath, receivedPath)
	assert.Equal(t, "20880001", receivedForm["pid"])
	assert.Equal(t, "MD5", receivedForm["sign_type"])
	assert.NotEmpty(t, receivedForm["timestamp"])
	assert.NotEmpty(t, receivedForm["sign"])
	assert.Equal(t, "P1", response.Data.TradeNo)
	assert.Equal(t, "https://cashier.example.com/pay", response.Data.PayURL)
}

func TestXznPayCreateOrderRejectsInvalidResponses(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "http error", statusCode: http.StatusBadGateway, body: `{}`},
		{name: "business error", statusCode: http.StatusOK, body: `{"code":0,"msg":"denied"}`},
		{name: "invalid json", statusCode: http.StatusOK, body: `{`},
		{name: "missing pay url", statusCode: http.StatusOK, body: `{"code":1,"msg":"success","data":{}}`},
		{name: "mismatched order", statusCode: http.StatusOK, body: `{"code":1,"msg":"success","data":{"trade_no":"P1","out_trade_no":"OTHER","pay_url":"https://pay.example.com"}}`},
		{name: "unsafe pay url", statusCode: http.StatusOK, body: `{"code":1,"msg":"success","data":{"trade_no":"P1","out_trade_no":"ORDER1","pay_url":"javascript:alert(1)"}}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(testCase.statusCode)
				_, _ = fmt.Fprint(w, testCase.body)
			}))
			defer server.Close()
			client, err := NewXznPayClient(XznPayConfig{
				GatewayURL: server.URL,
				PID:        "20880001",
				SignType:   "MD5",
				MD5Key:     "secret",
			}, server.Client())
			require.NoError(t, err)
			_, err = client.CreateOrder(context.Background(), map[string]string{"out_trade_no": "ORDER1"})
			require.Error(t, err)
		})
	}
}
