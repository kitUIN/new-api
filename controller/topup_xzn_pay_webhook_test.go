package controller

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseXznPayWebhookParams(t *testing.T) {
	expected := map[string]string{
		"pid":          "20880001",
		"trade_no":     "PAY1",
		"out_trade_no": "ORDER1",
		"sign":         "ABC123",
	}

	t.Run("url encoded", func(t *testing.T) {
		values := url.Values{}
		for key, value := range expected {
			values.Set(key, value)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/xzn-pay/webhook?ignored=query", strings.NewReader(values.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		params, err := parseXznPayWebhookParams(request)
		require.NoError(t, err)
		require.Equal(t, expected, params)
	})

	t.Run("multipart", func(t *testing.T) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for key, value := range expected {
			require.NoError(t, writer.WriteField(key, value))
		}
		require.NoError(t, writer.Close())

		request := httptest.NewRequest(http.MethodPost, "/api/xzn-pay/webhook?ignored=query", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())

		params, err := parseXznPayWebhookParams(request)
		require.NoError(t, err)
		require.Equal(t, expected, params)
	})
}
