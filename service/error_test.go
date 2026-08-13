package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestRelayErrorHandlerReturnsNonJSONResponseBodyWhenRequested(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader("image prompt rejected")),
	}

	err := RelayErrorHandler(context.Background(), resp, true)

	require.Equal(t, http.StatusBadRequest, err.StatusCode)
	require.Equal(t, "image prompt rejected", err.ToOpenAIError().Message)
	require.Equal(t, "image prompt rejected", err.ResponseBody)
}

func TestRelayErrorHandlerUsesStatusForEmptyResponseBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader("")),
	}

	err := RelayErrorHandler(context.Background(), resp, true)

	require.Equal(t, "bad response status code 502", err.ToOpenAIError().Message)
}

func TestRelayErrorHandlerPreservesRawBodyWhileLimitingClientMessage(t *testing.T) {
	message := strings.Repeat("upstream error ", 50)
	responseBody := `{"error":{"message":"` + message + `","type":"invalid_request_error","code":"bad_request"}}`
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
	}

	err := RelayErrorHandler(context.Background(), resp, false)

	require.Equal(t, responseBody, err.ResponseBody)
	require.Equal(t, message, err.ToOpenAIError().Message)
	require.Len(t, []rune(err.ToOpenAIErrorForResponse().Message), types.MaxErrorMessageRunes)
	require.Equal(t, message, err.ToOpenAIError().Message)
}
