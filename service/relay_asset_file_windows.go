package service

import (
	"golang.org/x/sys/windows"
	"os"
)

func openRelayAssetFile(path string, capacity int64) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), path)
	if err = reserveRelayAssetFile(f, capacity); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
