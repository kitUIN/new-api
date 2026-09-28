//go:build !windows

package service

import (
	"golang.org/x/sys/unix"
	"os"
)

func openRelayAssetFile(path string, capacity int64) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	// Write the full extent: unlike Truncate, this reserves real disk blocks on
	// all supported Unix filesystems, including those without fallocate.
	err = reserveRelayAssetFile(f, capacity)
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
