package middlewares

import (
	"go_distributed_social_media/internal/gateway"
	"net/http"
)

func IPRateLimiterMiddleware(next http.Handler) http.Handler {
	rl := gateway.NewLimiter(10, 20)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limiter := rl.GerLimiter(r.RemoteAddr)
		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
