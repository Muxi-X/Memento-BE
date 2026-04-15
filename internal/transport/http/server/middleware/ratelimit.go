package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// 限流中间件

type RateLimitConfig struct {
	RPS      int                       // 每个 IP 每秒允许的请求数
	Burst    int                       // 短时间内最多可存多少令牌
	KeyFunc  func(*gin.Context) string // 决定根据什么限流的函数，当前是 ClientIP()
	EntryTTL time.Duration             // 长时间未访问 key 清理时间
}

// 某个 key 的限流状态
type clientLimiter struct {
	limiter  *rate.Limiter // 做限流判断的对象
	lastSeen time.Time     // 上一次访问时间，用来清理长时间未使用 key
}

// 限流仓库
type limiterStore struct {
	mu          sync.Mutex
	clients     map[string]*clientLimiter
	lastCleanup time.Time
}

// 根据配置创建限流器，返回 Gin 中间件函数
func RateLimit(cfg RateLimitConfig) gin.HandlerFunc {
	// 关闭限流的逻辑
	if cfg.RPS <= 0 || cfg.Burst <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	// 默认按客户端 IP 限流
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(c *gin.Context) string { return c.ClientIP() }
	}
	if cfg.EntryTTL <= 0 {
		cfg.EntryTTL = 10 * time.Minute
	}

	// 创建独立的内存状态表
	store := &limiterStore{
		clients: make(map[string]*clientLimiter),
	}
	// 将 int 转换成 rate.Limit 类型，供rate.Limiter使用
	limit := rate.Limit(cfg.RPS)

	// 限流中间件函数
	return func(c *gin.Context) {
		key := cfg.KeyFunc(c)
		if key == "" {
			key = "unknown"
		}

		// 获取 key 对应的 limiter
		// 若 key 首次出现，创建新 limiter
		// 顺便更新 lastSeen，必要时会清理过期 key
		lim := store.get(key, limit, cfg.Burst, cfg.EntryTTL)
		// Allow() 尝试获取一个令牌，若有足够令牌返回 true，否则 false
		if !lim.Allow() {
			// 被限流的响应
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":       "rate_limited",
				"reason":     "common.rate_limited",
				"message":    "too many requests",
				"request_id": GetRequestID(c),
			})
			return
		}
		c.Next()
	}
}

func (s *limiterStore) get(key string, r rate.Limit, b int, ttl time.Duration) *rate.Limiter {
	// 用于更新 lastSeen 和 TTL 判断
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.clients == nil {
		s.clients = make(map[string]*clientLimiter)
	}

	// 获得 key
	cl, ok := s.clients[key]
	// 若第一次得到该key，新建一个 Limiter
	if !ok {
		cl = &clientLimiter{limiter: rate.NewLimiter(r, b), lastSeen: now}
		s.clients[key] = cl
	} else {
		cl.lastSeen = now
	}

	// 如果距上次清理 key 的时间超过 TTL，才检测是否有过期 key
	if now.Sub(s.lastCleanup) > ttl {
		for k, v := range s.clients {
			// 只删除过期的 key
			if now.Sub(v.lastSeen) > ttl {
				delete(s.clients, k)
			}
		}
		s.lastCleanup = now
	}

	return cl.limiter
}
