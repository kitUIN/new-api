package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestBuildTieredTokenParamsSeparatesImageOutput(t *testing.T) {
	usage := &dto.Usage{
		CompletionTokens: 100,
		CompletionTokenDetails: dto.OutputTokenDetails{
			ImageTokens: 25,
		},
	}

	params := BuildTieredTokenParams(usage, false, map[string]bool{"img_o": true})
	require.Equal(t, float64(75), params.C)
	require.Equal(t, float64(25), params.ImgO)
}
