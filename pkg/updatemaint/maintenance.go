// Package updatemaint keeps the gateway's HTTP listener available across a binary update.
package updatemaint

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	helperArg      = "--serve-update-maintenance"
	HandoffVersion = "update-listener-v1"
)

var (
	publicListener *net.TCPListener // Set before the HTTP server accepts its first request.
	activeNotice   atomic.Pointer[notice]
)

type notice struct {
	deadline time.Time
	message  string
	json     []byte
}

func newNotice(deadline time.Time) *notice {
	message := fmt.Sprintf("系统正在升级，预计%s完成，请等待升级完成后再重试", deadline.Format(time.DateTime))
	body, _ := common.Marshal(map[string]any{
		"success": false, "message": message,
		"error": map[string]string{"message": message, "type": "service_unavailable"},
	})
	return &notice{deadline: deadline, message: message, json: body}
}

func (n *notice) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if time.Now().Before(n.deadline) {
		w.Header().Set("Retry-After", n.deadline.UTC().Format(http.TimeFormat))
	} else {
		w.Header().Set("Retry-After", "5")
	}
	w.Header().Set("Connection", "close")
	apiRequest := strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") || strings.HasPrefix(r.URL.Path, "/v1beta/")
	if apiRequest || strings.Contains(r.Header.Get("Accept"), "application/json") || strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write(n.json)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = fmt.Fprint(w, n.message)
}

func Serve(addr string, app http.Handler) error {
	listener, err := listen(addr)
	if err != nil {
		return err
	}
	publicListener = listener
	return http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n := activeNotice.Load(); n != nil {
			n.ServeHTTP(w, r)
			return
		}
		app.ServeHTTP(w, r)
	}))
}

func Start(executable string, deadline time.Time) error {
	if publicListener == nil {
		return fmt.Errorf("gateway listener is unavailable")
	}
	n := newNotice(deadline)
	if !activeNotice.CompareAndSwap(nil, n) {
		return fmt.Errorf("a system update is already in progress")
	}
	if err := startHelper(executable, deadline, publicListener); err != nil {
		activeNotice.Store(nil)
		return err
	}
	return nil
}

func Resume() {
	if err := StopHelper(); err != nil {
		common.SysError("failed to stop update maintenance: " + err.Error())
	}
	activeNotice.Store(nil)
}

// HandleCommand runs before database, cache, and application initialization.
func HandleCommand(args []string) bool {
	if len(args) == 2 && args[0] == "--version" && args[1] == HandoffVersion {
		fmt.Println(HandoffVersion)
		return false // The existing --version handler also prints the release version.
	}
	if len(args) == 0 || args[0] != helperArg {
		return false
	}
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "missing maintenance deadline")
		os.Exit(1)
	}
	deadline, err := time.Parse(time.RFC3339, args[1])
	if err == nil {
		err = runHelper(newNotice(deadline))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return true
}
