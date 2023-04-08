package cachecenter

import (
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
)

var C *cache.Cache

func init() {
	// 设置超时时间和清理时间
	C = cache.New(5*time.Minute, 10*time.Minute)
}

// 删除匹配某个前缀的缓存项
func deleteCacheByPrefix(c *cache.Cache, prefix string) {
	keys := c.Items()
	for k := range keys {
		if strings.HasPrefix(k, prefix) {
			c.Delete(k)
			println("delete cache:", k)
		}
	}
}
