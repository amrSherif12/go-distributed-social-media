package middlewares

import (
	"go_distributed_social_media/internal/common/config"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_KEY", "yRpuW4VG75ABOKtgmaUj4xPmes7K6GHLRvysObWuzxh")
	code := m.Run()
	os.Exit(code)
}

func generateTestToken(uid string, ttl time.Duration) string {
	claims := jwt.MapClaims{
		"uid": uid,
		"exp": time.Now().Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(config.Required(config.JWTKey)))
	return tokenStr
}

func TestJWTMiddleware(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := r.Context().Value("uid").(string)
		if !ok || uid == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		assert.Equal(t, "user_123", uid)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(uid))
	})

	jwtMiddleware := AuthMiddleware(nextHandler)

	t.Run("Valid Token - Returns 200 and extracts UID", func(t *testing.T) {
		token := generateTestToken("user_123", 10*time.Minute)

		req := httptest.NewRequest("GET", "/profile", nil)
		req.AddCookie(&http.Cookie{
			Name:  "auth",
			Value: token,
		})
		rec := httptest.NewRecorder()

		jwtMiddleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Expired Token - Returns 401", func(t *testing.T) {
		token := generateTestToken("user_123", -10*time.Minute)

		req := httptest.NewRequest("GET", "/api/v1/auth/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		jwtMiddleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Missing Header - Returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/auth/profile", nil)
		rec := httptest.NewRecorder()

		jwtMiddleware.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
