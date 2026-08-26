package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRankingCustomTimeRange(t *testing.T) {
	tests := []struct {
		name      string
		period    string
		target    string
		wantStart int64
		wantEnd   int64
		wantError string
	}{
		{name: "non custom ignores range", period: "week", target: "/api/rankings?start_time=bad"},
		{name: "valid range", period: "custom", target: "/api/rankings?start_time=100&end_time=200", wantStart: 100, wantEnd: 200},
		{name: "missing start", period: "custom", target: "/api/rankings?end_time=200", wantError: "invalid ranking start_time"},
		{name: "invalid end", period: "custom", target: "/api/rankings?start_time=100&end_time=bad", wantError: "invalid ranking end_time"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Request = httptest.NewRequest(http.MethodGet, test.target, nil)

			startTime, endTime, err := rankingCustomTimeRange(context, test.period)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.wantStart, startTime)
			require.Equal(t, test.wantEnd, endTime)
		})
	}
}
