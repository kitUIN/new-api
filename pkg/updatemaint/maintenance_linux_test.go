package updatemaint

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMaintenanceListenerHandoff(t *testing.T) {
	// Unix socket paths have a short kernel length limit, including the temp directory.
	dir, err := os.MkdirTemp("", "handoff-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "control")
	control, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	require.NoError(t, err)
	defer control.Close()
	done := make(chan error, 1)
	go func() { done <- serveUntilHandoff(control, newNotice(time.Now().Add(3*time.Minute))) }()

	original, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer original.Close()
	url := "http://" + original.Addr().String()
	parent, err := dialControl(path)
	require.NoError(t, err)
	require.NoError(t, sendListener(parent, original))
	parent.Close()
	original.Close()

	// A failed startup must leave the helper serving until a replacement acknowledges the FD.
	aborted, err := dialControl(path)
	require.NoError(t, err)
	unused, err := receiveListener(aborted)
	require.NoError(t, err)
	unused.Close()
	aborted.Close()
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	next, err := dialControl(path)
	require.NoError(t, err)
	reclaimed, err := receiveListener(next)
	require.NoError(t, err)
	defer reclaimed.Close()
	_, err = next.Write([]byte{1})
	require.NoError(t, err)
	next.Close()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(controlTimeout):
		t.Fatal("helper did not exit after handoff")
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "ready")
	})}
	defer server.Close()
	go server.Serve(reclaimed)
	resp, err = client.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ready", string(body))
}
