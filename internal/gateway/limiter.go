package gateway

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	clients map[string]*Client
	mu      sync.RWMutex
	r       rate.Limit
	b       int
}

func NewLimiter(r rate.Limit, b int) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*Client),
		r:       r,
		b:       b,
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) GerLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	c, exists := rl.clients[key]
	if !exists {
		c = &Client{
			limiter: rate.NewLimiter(rl.r, rl.b),
		}
		rl.clients[key] = c
	}

	c.lastSeen = time.Now()
	return c.limiter
}

func (rl *RateLimiter) cleanupLoop() {
	for {
		time.Sleep(time.Minute * 10)
		rl.mu.Lock()
		for key, c := range rl.clients {
			if time.Since(c.lastSeen) > time.Minute*10 {
				delete(rl.clients, key)
			}
		}
		rl.mu.Unlock()
	}
}
