package fetch

import (
	"strconv"
	"sync"
)

// fetchDedup 记录一次任务内已经请求过的实体
type fetchDedup struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func newFetchDedup() *fetchDedup {
	return &fetchDedup{seen: map[string]struct{}{}}
}

// claim 返回 true 表示本次任务第一次领取该实体
func (d *fetchDedup) claim(kind string, id int) bool {
	if d == nil {
		return true
	}
	key := kind + ":" + strconv.Itoa(id)
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.seen[key]; ok {
		return false
	}
	d.seen[key] = struct{}{}
	return true
}
