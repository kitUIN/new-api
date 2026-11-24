package updatemaint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	mainUnit       = "nachoai.service"
	helperName     = "nachoai-update-maintenance"
	controlPath    = "/run/" + helperName + "/listener.sock"
	controlTimeout = 10 * time.Second
)

// The systemd-owned 0700 runtime directory restricts access to the listener FD.
// Only the gateway instance managed by the updater may reclaim it on startup.
func isMainProcess() bool {
	ctx, cancel := context.WithTimeout(context.Background(), controlTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemctl", "show", "--property=MainPID", "--value", mainUnit).Output()
	return err == nil && strings.TrimSpace(string(out)) == strconv.Itoa(os.Getpid())
}

func listen(addr string) (*net.TCPListener, error) {
	address, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(controlPath); err != nil || !isMainProcess() {
		return net.ListenTCP("tcp", address)
	}
	return reclaimListener(address)
}

func reclaimListener(address *net.TCPAddr) (*net.TCPListener, error) {
	control, err := dialControl(controlPath)
	if err != nil {
		return nil, err
	}
	defer control.Close()
	listener, err := receiveListener(control)
	if err != nil {
		return nil, err
	}
	// The administrator may also have changed PORT for the new service.
	if address != nil && address.Port != 0 && listener.Addr().(*net.TCPAddr).Port != address.Port {
		listener.Close()
		listener, err = net.ListenTCP("tcp", address)
		if err != nil {
			return nil, err
		}
	}
	if _, err := control.Write([]byte{1}); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

func startHelper(executable string, deadline time.Time, listener *net.TCPListener) error {
	if !isMainProcess() {
		return fmt.Errorf("self-update must run in %s", mainUnit)
	}
	// A normal child would be killed with the old service's cgroup on restart.
	// systemd-run returns once the helper has opened its private control socket.
	ctx, cancel := context.WithTimeout(context.Background(), 2*controlTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemd-run", "--quiet", "--collect", "--unit="+helperName,
		"--service-type=notify", "--property=TimeoutStartSec=10s", "--property=RuntimeDirectory="+helperName,
		"--property=RuntimeDirectoryMode=0700", "--uid="+strconv.Itoa(os.Geteuid()),
		executable, helperArg, deadline.Format(time.RFC3339)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("start update maintenance: %w: %s", err, strings.TrimSpace(string(out)))
	}
	control, err := dialControl(controlPath)
	if err == nil {
		defer control.Close()
		err = sendListener(control, listener)
	}
	if err != nil {
		return errors.Join(err, StopHelper())
	}
	return nil
}

func StopHelper() error {
	if _, err := os.Stat(controlPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), controlTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "systemctl", "stop", helperName+".service").CombinedOutput()
	if err == nil {
		return nil
	}
	// A failed systemctl restart can also make stop unavailable. The original
	// gateway still owns its copy and can dismiss the helper via a normal handoff.
	listener, handoffErr := reclaimListener(nil)
	if handoffErr == nil {
		return listener.Close()
	}
	if errors.Is(handoffErr, os.ErrNotExist) {
		return nil
	}
	return errors.Join(fmt.Errorf("stop %s: %w: %s", helperName, err, strings.TrimSpace(string(out))), handoffErr)
}

func dialControl(path string) (*net.UnixConn, error) {
	conn, err := net.DialTimeout("unix", path, controlTimeout)
	if err != nil {
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(controlTimeout))
	return conn.(*net.UnixConn), nil
}

func sendListener(conn *net.UnixConn, listener *net.TCPListener) error {
	file, err := listener.File()
	if err != nil {
		return err
	}
	defer file.Close()
	if _, _, err = conn.WriteMsgUnix([]byte{1}, unix.UnixRights(int(file.Fd())), nil); err != nil {
		return err
	}
	var ack [1]byte
	if _, err = io.ReadFull(conn, ack[:]); err != nil {
		return err
	}
	if ack[0] != 1 {
		return errors.New("listener handoff was rejected")
	}
	return nil
}

func receiveListener(conn *net.UnixConn) (*net.TCPListener, error) {
	var payload [1]byte
	rights := make([]byte, unix.CmsgSpace(4))
	_, n, _, _, err := conn.ReadMsgUnix(payload[:], rights)
	if err != nil {
		return nil, err
	}
	messages, err := unix.ParseSocketControlMessage(rights[:n])
	if err != nil || len(messages) != 1 {
		return nil, errors.New("missing listener control message")
	}
	fds, err := unix.ParseUnixRights(&messages[0])
	if err != nil || len(fds) != 1 {
		for _, fd := range fds {
			unix.Close(fd)
		}
		return nil, errors.New("invalid listener descriptor")
	}
	file := os.NewFile(uintptr(fds[0]), "gateway-listener")
	defer file.Close()
	ln, err := net.FileListener(file)
	if err != nil {
		return nil, err
	}
	tcp, ok := ln.(*net.TCPListener)
	if !ok {
		ln.Close()
		return nil, errors.New("expected a TCP listener")
	}
	return tcp, nil
}

func runHelper(n *notice) error {
	control, err := net.ListenUnix("unix", &net.UnixAddr{Name: controlPath, Net: "unix"})
	if err != nil {
		return err
	}
	defer control.Close()
	notify, err := net.Dial("unixgram", os.Getenv("NOTIFY_SOCKET"))
	if err != nil {
		return err
	}
	_, err = notify.Write([]byte("READY=1"))
	notify.Close()
	if err != nil {
		return err
	}
	return serveUntilHandoff(control, n)
}

func serveUntilHandoff(control *net.UnixListener, n *notice) error {
	control.SetDeadline(time.Now().Add(controlTimeout))
	parent, err := control.AcceptUnix()
	if err != nil {
		return err
	}
	parent.SetDeadline(time.Now().Add(controlTimeout))
	listener, err := receiveListener(parent)
	if err != nil {
		parent.Close()
		return err
	}
	defer listener.Close()
	server := &http.Server{Handler: n, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("serving update maintenance: %v", err)
			control.Close()
		}
	}()
	_, err = parent.Write([]byte{1})
	parent.Close()
	if err != nil {
		return err
	}
	control.SetDeadline(time.Time{})
	for {
		next, err := control.AcceptUnix()
		if err != nil {
			return err
		}
		next.SetDeadline(time.Now().Add(controlTimeout))
		err = sendListener(next, listener)
		next.Close()
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return server.Shutdown(ctx)
		}
		log.Printf("gateway listener handoff: %v", err)
	}
}
