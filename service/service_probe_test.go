package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServiceProbeRead(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-private-key", r.Header.Get("Authorization"))
		path = r.URL.Path
		if strings.HasSuffix(path, "/history") {
			from, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("from"))
			require.NoError(t, err)
			to, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("to"))
			require.NoError(t, err)
			require.Equal(t, 24*time.Hour, to.Sub(from))
			require.True(t, to.Before(time.Now()))
		}
		fmt.Fprint(w, `{"totalBytes":0,"limitBytes":0,"receivedBytes":null,"online":false}`)
	}))
	defer server.Close()
	t.Setenv("GREATMAO_HIVE_URL", server.URL+"/hive/")
	t.Setenv("GREATMAO_HIVE_READ_API_KEY", "test-private-key")
	data, err := ReadServiceProbe(context.Background(), "quotas", "", "")
	require.NoError(t, err)
	require.Equal(t, "/hive/api/v1/bees/traffic/quotas", path)
	require.Contains(t, string(data), `"limitBytes":0`)
	require.Contains(t, string(data), `"receivedBytes":null`)
	_, err = ReadServiceProbe(context.Background(), "history", "bee #1", "24h")
	require.NoError(t, err)
	require.Equal(t, "/hive/api/v1/bees/bee #1/online/history", path)
	for _, id := range []string{"", "..", "bee/../../admin", "bee\\admin"} {
		_, err = ReadServiceProbe(context.Background(), "current", id, "")
		require.EqualError(t, err, "serviceProbe.invalidRequest")
	}
	_, err = ReadServiceProbe(context.Background(), "history", "bee", "365d")
	require.EqualError(t, err, "serviceProbe.invalidRequest")
}

func TestServiceProbeUnavailableAndErrors(t *testing.T) {
	for _, status := range []int{404, 503, 504} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
			defer server.Close()
			t.Setenv("GREATMAO_HIVE_URL", server.URL)
			t.Setenv("GREATMAO_HIVE_READ_API_KEY", "private-key")
			data, err := ReadServiceProbe(context.Background(), "current", "bee", "")
			require.NoError(t, err)
			require.Nil(t, data)
			_, err = ReadServiceProbe(context.Background(), "quotas", "", "")
			require.EqualError(t, err, "serviceProbe.upstreamFailed")
		})
	}
	t.Setenv("GREATMAO_HIVE_URL", "")
	t.Setenv("GREATMAO_HIVE_READ_API_KEY", "")
	_, err := ReadServiceProbe(context.Background(), "quotas", "", "")
	require.ErrorIs(t, err, ErrServiceProbeNotConfigured)
}

func TestServiceProbeRejectsRedirectAndInvalidJSON(t *testing.T) {
	var redirected bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	t.Setenv("GREATMAO_HIVE_URL", server.URL)
	t.Setenv("GREATMAO_HIVE_READ_API_KEY", "private-key")
	_, err := ReadServiceProbe(context.Background(), "quotas", "", "")
	require.EqualError(t, err, "serviceProbe.upstreamFailed")
	require.False(t, redirected)
	server.Close()
	for _, body := range []string{"<html>not json</html>", "null", "[]", `{"value":"` + strings.Repeat("x", 4<<20) + `"}`} {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		t.Setenv("GREATMAO_HIVE_URL", server.URL)
		_, err = ReadServiceProbe(context.Background(), "quotas", "", "")
		require.EqualError(t, err, "serviceProbe.invalidResponse")
		server.Close()
	}
}
