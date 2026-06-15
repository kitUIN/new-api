package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestRedactSensitiveHeadersJSON(t *testing.T) {
	headers := map[string][]string{
		"Authorization": {"Bearer secret"},
		"Mj-Api-Secret": {"mj-secret"},
		"X-Trace-Id":    {"trace-123"},
	}
	raw, err := common.Marshal(headers)
	require.NoError(t, err)

	redacted := RedactSensitiveHeadersJSON(string(raw))

	var got map[string][]string
	require.NoError(t, common.Unmarshal([]byte(redacted), &got))
	require.Equal(t, []string{"[redacted]"}, got["Authorization"])
	require.Equal(t, []string{"[redacted]"}, got["Mj-Api-Secret"])
	require.Equal(t, []string{"trace-123"}, got["X-Trace-Id"])
}
