package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildGeminiChannelTestRequestUsesNativeFormat(t *testing.T) {
	request := buildTestRequest("gemini-2.5-flash", string(constant.EndpointTypeGemini), nil, true)
	geminiRequest, ok := request.(*dto.GeminiChatRequest)
	require.True(t, ok)
	require.Len(t, geminiRequest.Contents, 1)
	assert.Equal(t, "user", geminiRequest.Contents[0].Role)
	require.NotNil(t, geminiRequest.GenerationConfig.MaxOutputTokens)
	assert.Equal(t, uint(3000), *geminiRequest.GenerationConfig.MaxOutputTokens)
}

func TestNormalizeGeminiChannelTestStreamPath(t *testing.T) {
	path := "/v1beta/models/gemini-2.5-flash:generateContent"
	assert.Equal(t,
		"/v1beta/models/gemini-2.5-flash:streamGenerateContent",
		normalizeChannelTestRequestPath(path, string(constant.EndpointTypeGemini), true),
	)
	assert.Equal(t, path, normalizeChannelTestRequestPath(path, string(constant.EndpointTypeGemini), false))
}
