//go:build !linux

package updatemaint

import (
	"errors"
	"net"
	"time"
)

func listen(addr string) (*net.TCPListener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return ln.(*net.TCPListener), nil
}

var errUnsupported = errors.New("update maintenance requires Linux")

func startHelper(string, time.Time, *net.TCPListener) error { return errUnsupported }
func runHelper(*notice) error                               { return errUnsupported }
func StopHelper() error                                     { return errUnsupported }
