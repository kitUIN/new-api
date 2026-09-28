package service

import "os"

func reserveRelayAssetFile(f *os.File, capacity int64) error {
	// Rewriting also verifies allocation of existing sparse or partially created files.
	block := make([]byte, 1<<20)
	for offset := int64(0); offset < capacity; {
		next := min(int64(len(block)), capacity-offset)
		if _, err := f.WriteAt(block[:next], offset); err != nil {
			return err
		}
		offset += next
	}
	if err := f.Truncate(capacity); err != nil {
		return err
	}
	return f.Sync()
}
