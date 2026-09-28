package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

type relayAssetEntry struct {
	id, mime, ext string
	offset, size  int64
	expires, used time.Time
	leases        int
}

// RelayImageCache owns one exclusively opened, fully allocated file. Offsets are
// reused only after all upstream and download leases have been released.
type RelayImageCache struct {
	mu        sync.Mutex
	file      *os.File
	entries   map[string]*relayAssetEntry
	key       [32]byte
	capacity  int64
	ttl       time.Duration
	stop      chan struct{}
	closeOnce sync.Once
}

var RelayAssets *RelayImageCache

func NewRelayImageCache(dir string, capacity int64, ttl time.Duration) (*RelayImageCache, error) {
	if capacity <= 0 || ttl <= 0 {
		return nil, fmt.Errorf("invalid relay asset cache configuration")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	f, err := openRelayAssetFile(filepath.Join(dir, "assets.bin"), capacity)
	if err != nil {
		return nil, err
	}
	c := &RelayImageCache{file: f, entries: make(map[string]*relayAssetEntry), capacity: capacity, ttl: ttl, stop: make(chan struct{})}
	if _, err = rand.Read(c.key[:]); err != nil {
		f.Close()
		return nil, err
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.cleanup()
			case <-c.stop:
				return
			}
		}
	}()
	return c, nil
}

func InitRelayImageCache() error {
	cfg := *operation_setting.GetRelayAssetSetting()
	for name, target := range map[string]*int{"RELAY_IMAGE_CACHE_MB": &cfg.CacheMB, "RELAY_IMAGE_TTL_SECONDS": &cfg.TTLSeconds} {
		if value, ok := os.LookupEnv(name); ok {
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid %s", name)
			}
			*target = n
		}
	}
	if value, ok := os.LookupEnv("RELAY_IMAGE_CACHE_DIR"); ok {
		cfg.CacheDir = value
	}
	if cfg.CacheMB < 1 || cfg.CacheMB > 16384 || cfg.TTLSeconds < 60 || cfg.TTLSeconds > 86400 || strings.TrimSpace(cfg.CacheDir) == "" {
		return fmt.Errorf("invalid relay asset cache settings")
	}
	var err error
	RelayAssets, err = NewRelayImageCache(cfg.CacheDir, int64(cfg.CacheMB)<<20, time.Duration(cfg.TTLSeconds)*time.Second)
	return err
}

func (c *RelayImageCache) Close() error {
	var err error
	c.closeOnce.Do(func() { close(c.stop); c.mu.Lock(); defer c.mu.Unlock(); err = c.file.Close() })
	return err
}

func (c *RelayImageCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, e := range c.entries {
		if e.leases == 0 && !time.Now().Before(e.expires) {
			delete(c.entries, id)
		}
	}
}

func (c *RelayImageCache) gap(size int64) (int64, bool) {
	entries := make([]*relayAssetEntry, 0, len(c.entries))
	for _, e := range c.entries {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].offset < entries[j].offset })
	var pos int64
	for _, e := range entries {
		if e.offset-pos >= size {
			return pos, true
		}
		pos = e.offset + e.size
	}
	return pos, c.capacity-pos >= size
}

func (c *RelayImageCache) sign(path, expires string) string {
	h := hmac.New(sha256.New, c.key[:])
	h.Write([]byte(path + "\n" + expires))
	return hex.EncodeToString(h.Sum(nil))
}

func RelayAssetBaseURL() string {
	base := os.Getenv("RELAY_IMAGE_BASE_URL")
	if base == "" {
		base = system_setting.ServerAddress
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return ""
	}
	return strings.TrimRight(base, "/")
}

func (c *RelayImageCache) Put(user int, mime, ext string, data []byte, base string) (string, func(), bool) {
	if c == nil || base == "" || int64(len(data)) > c.capacity {
		return "", nil, false
	}
	h := sha256.New()
	fmt.Fprintf(h, "%d\x00%s\x00", user, mime)
	h.Write(data)
	id := hex.EncodeToString(h.Sum(nil))
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[id]
	if e == nil {
		offset, ok := c.gap(int64(len(data)))
		for !ok {
			var victim *relayAssetEntry
			for _, candidate := range c.entries {
				if candidate.leases == 0 && (victim == nil || candidate.used.Before(victim.used)) {
					victim = candidate
				}
			}
			if victim == nil {
				return "", nil, false
			}
			delete(c.entries, victim.id)
			offset, ok = c.gap(int64(len(data)))
		}
		if n, err := c.file.WriteAt(data, offset); err != nil || n != len(data) {
			return "", nil, false
		}
		e = &relayAssetEntry{id: id, mime: mime, ext: ext, offset: offset, size: int64(len(data))}
		c.entries[id] = e
	}
	e.used = time.Now()
	e.expires = e.used.Add(c.ttl)
	e.leases++
	path := "/relay-files/" + id + e.ext
	if strings.HasPrefix(mime, "image/") {
		path = "/relay-images/" + id
	}
	expires := strconv.FormatInt(e.expires.Unix(), 10)
	var once sync.Once
	release := func() { once.Do(func() { c.mu.Lock(); e.leases--; c.mu.Unlock() }) }
	return base + path + "?expires=" + expires + "&signature=" + c.sign(path, expires), release, true
}

func (c *RelayImageCache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if c == nil {
		http.NotFound(w, r)
		return
	}
	path := r.URL.Path
	expires := r.URL.Query().Get("expires")
	expiry, err := strconv.ParseInt(expires, 10, 64)
	sig, sigErr := hex.DecodeString(r.URL.Query().Get("signature"))
	expected, _ := hex.DecodeString(c.sign(path, expires))
	if err != nil || sigErr != nil || time.Now().Unix() >= expiry || !hmac.Equal(sig, expected) {
		http.NotFound(w, r)
		return
	}
	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	c.mu.Lock()
	e := c.entries[id]
	if e == nil {
		c.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	e.leases++
	e.used = time.Now()
	c.mu.Unlock()
	defer func() { c.mu.Lock(); e.leases--; c.mu.Unlock() }()
	w.Header().Set("Content-Type", e.mime)
	if !strings.HasPrefix(e.mime, "image/") {
		w.Header().Set("Content-Disposition", `attachment; filename="asset`+e.ext+`"`)
	}
	http.ServeContent(w, r, "asset"+e.ext, time.Time{}, io.NewSectionReader(c.file, e.offset, e.size))
}

func (c *RelayImageCache) Stats() map[string]int64 {
	stats := map[string]int64{"entries": 0, "payload_bytes": 0, "capacity_bytes": 0}
	if c == nil {
		return stats
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	stats["entries"] = int64(len(c.entries))
	stats["capacity_bytes"] = c.capacity
	for _, e := range c.entries {
		stats["payload_bytes"] += e.size
	}
	return stats
}
