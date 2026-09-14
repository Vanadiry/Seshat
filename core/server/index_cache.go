package server

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

// cacheKey 生成带类型名的缓存键
func cacheKey[T any](path string) string {
	return fmt.Sprintf("%s:%T", path, *new(T))
}

// loadCachedIndex 读取并缓存索引 JSON 文件，mtime 未变则返回缓存
func loadCachedIndex[T any](path string) (T, error) {
	var zero T

	stat, err := os.Stat(path)
	if err != nil {
		return zero, err
	}

	key := cacheKey[T](path)
	indexCacheMu.RLock()
	entry, ok := indexCache[key]
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
	indexCache[key] = indexCacheEntry{modTime: stat.ModTime(), data: result}
	indexCacheMu.Unlock()

	return result, nil
}

// clearIndexCache 清除指定文件路径的全部类型缓存
func clearIndexCache(path string) {
	prefix := path + ":"
	indexCacheMu.Lock()
	for k := range indexCache {
		if strings.HasPrefix(k, prefix) {
			delete(indexCache, k)
		}
	}
	indexCacheMu.Unlock()
}
