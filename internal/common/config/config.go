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

	PostSrvHost string = "POST_SRV_HOST"
	PostSrvPort string = "POST_SRV_PORT"

	PostDBName    string = "POST_DB_NAME"
	PostDBHost    string = "POST_DB_HOST"
	PostDBPort    string = "POST_DB_PORT"
	PostDBPass    string = "POST_DB_PASS"
	PostDBUser    string = "POST_DB_USER"
	PostCacheHost string = "POST_CACHE_HOST"
	PostCachePort string = "POST_CACHE_PORT"

	PostEventBrokerHost string = "POST_EVENT_BROKER_HOST"
	PostEventBrokerPort string = "POST_EVENT_BROKER_PORT"
	PostEventBrokerUser string = "POST_EVENT_BROKER_USER"
	PostEventBrokerPass string = "POST_EVENT_BROKER_PASS"
	PostEventQueue      string = "POST_EVENT_QUEUE"

	PostObjDBHost       string = "POST_OBJ_DB_HOST"
	PostObjDBPublicHost string = "POST_OBJ_DB_PUBLIC_HOST"
	PostObjDBPort       string = "POST_OBJ_DB_PORT"
	PostObjDBUser       string = "POST_OBJ_DB_USER"
	PostObjDBName       string = "POST_OBJ_DB_NAME"
	PostObjDBPass       string = "POST_OBJ_DB_PASS"
	PostObjDBBucket     string = "POST_OBJ_DB_BUCKET"
	PostObjDBRegion     string = "POST_OBJ_DB_REGION"

	FollowSrvHost   string = "FOLLOW_SRV_HOST"
	FollowSrvPort   string = "FOLLOW_SRV_PORT"
	FollowDBHost    string = "FOLLOW_DB_HOST"
	FollowDBPort    string = "FOLLOW_DB_PORT"
	FollowDBUser    string = "FOLLOW_DB_USER"
	FollowDBPass    string = "FOLLOW_DB_PASS"
	FollowCacheHost string = "FOLLOW_CACHE_HOST"
	FollowCachePort string = "FOLLOW_CACHE_PORT"

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
