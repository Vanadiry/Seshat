package server

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

var (
	indexCache   = map[string]indexCacheEntry{}
	indexCacheMu sync.RWMutex
)

type indexCacheEntry struct {
	modTime time.Time
	data    any
}

// loadCachedIndex reads and caches an index JSON file. Returns cached data if
// the file's mtime hasn't changed since the last load.
func loadCachedIndex[T any](path string) (T, error) {
	var zero T

	stat, err := os.Stat(path)
	if err != nil {
		return zero, err
	}

	indexCacheMu.RLock()
	entry, ok := indexCache[path]
	indexCacheMu.RUnlock()
	if ok && entry.modTime.Equal(stat.ModTime()) {
		return entry.data.(T), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return zero, err
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return zero, err
	}

	indexCacheMu.Lock()
	indexCache[path] = indexCacheEntry{modTime: stat.ModTime(), data: result}
	indexCacheMu.Unlock()

	return result, nil
}

// clearIndexCache removes a specific index file from the cache (call after writing).
func clearIndexCache(path string) {
	indexCacheMu.Lock()
	delete(indexCache, path)
	indexCacheMu.Unlock()
}
