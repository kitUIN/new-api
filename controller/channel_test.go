package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseChannelTestStream(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "missing defaults to stream", url: "/api/channel/test/1", want: true},
		{name: "empty defaults to stream", url: "/api/channel/test/1?stream=", want: true},
		{name: "invalid defaults to stream", url: "/api/channel/test/1?stream=nope", want: true},
		{name: "explicit false disables stream", url: "/api/channel/test/1?stream=false", want: false},
		{name: "explicit true enables stream", url: "/api/channel/test/1?stream=true", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", tt.url, nil)

			if got := parseChannelTestStream(c); got != tt.want {
				t.Fatalf("parseChannelTestStream() = %v, want %v", got, tt.want)
			}
		})
	}
}
