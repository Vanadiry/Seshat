package index

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	cache   = map[string]cacheEntry{}
	cacheMu sync.RWMutex
)

type cacheEntry struct {
	modTime time.Time
	data    any
}

// cacheKey 生成带类型名的缓存键
func cacheKey[T any](path string) string {
	return fmt.Sprintf("%s:%T", path, *new(T))
}

// LoadCached 读取并缓存索引 JSON 文件，mtime 未变则返回缓存
func LoadCached[T any](path string) (T, error) {
	var zero T

	stat, err := os.Stat(path)
	if err != nil {
		return zero, err
	}

	key := cacheKey[T](path)
	cacheMu.RLock()
	entry, ok := cache[key]
	cacheMu.RUnlock()
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

	cacheMu.Lock()
	cache[key] = cacheEntry{modTime: stat.ModTime(), data: result}
	cacheMu.Unlock()

	return result, nil
}

// ClearCache 清除指定文件路径的全部类型缓存
func ClearCache(path string) {
	prefix := path + ":"
	cacheMu.Lock()
	for k := range cache {
		if strings.HasPrefix(k, prefix) {
			delete(cache, k)
		}
	}
	cacheMu.Unlock()
}
