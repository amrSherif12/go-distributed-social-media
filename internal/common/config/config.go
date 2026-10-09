package config

import (
	"fmt"
	"os"
	"sync"
)

const (
	JWTKey    string = "JWT_KEY"
	SQLDbUser string = "SQL_DB_USER"
	SQLDbPass string = "SQL_DB_PASS"

	UserSrvHost string = "USER_SRV_HOST"
	UserSrvPort string = "USER_SRV_PORT"

	UserDBHost string = "USER_DB_HOST"
	UserDBPort string = "USER_DB_PORT"
	UserDBName string = "USER_DB_NAME"

	GatewayHost string = "GATEWAY_HOST"
	GatewayPort string = "GATEWAY_PORT"
)

var (
	cache = make(map[string]string)
	mu    sync.RWMutex
)

func Get(key string) string {
	mu.RLock()
	if val, exists := cache[key]; exists {
		mu.RUnlock()
		return val
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	if val, exists := cache[key]; exists {
		return val
	}

	val := os.Getenv(key)
	cache[key] = val
	return val
}

func Required(key string) string {
	val := Get(key)
	if val == "" {
		panic(fmt.Sprintf("config error: required environment variable %s is missing", key))
	}
	return val
}
