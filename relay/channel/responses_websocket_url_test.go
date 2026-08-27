package channel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToWebSocketURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "https", input: "https://api.example.com/v1/responses?x=1", expected: "wss://api.example.com/v1/responses?x=1"},
		{name: "http", input: "http://127.0.0.1:8080/v1/responses", expected: "ws://127.0.0.1:8080/v1/responses"},
		{name: "already websocket", input: "wss://api.example.com/v1/responses", expected: "wss://api.example.com/v1/responses"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual, err := ToWebSocketURL(test.input)
			require.NoError(t, err)
			require.Equal(t, test.expected, actual)
		})
	}
}

func TestToWebSocketURLRejectsUnsupportedScheme(t *testing.T) {
	t.Parallel()
	_, err := ToWebSocketURL("ftp://api.example.com/v1/responses")
	require.ErrorContains(t, err, "unsupported websocket URL scheme")
}
