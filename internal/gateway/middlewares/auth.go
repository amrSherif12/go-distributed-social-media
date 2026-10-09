package middlewares

import (
	"context"
	"errors"
	"go_distributed_social_media/internal/common/config"
	"go_distributed_social_media/internal/common/utils"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("auth")
		if err != nil {
			_ = utils.WriteError(w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}
		token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (any, error) {
			return []byte(config.Required(config.JWTKey)), nil
		},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid {
			_ = utils.WriteError(w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			_ = utils.WriteError(w, http.StatusInternalServerError,
				errors.New("internal server error"))
			return
		}

		uid, _ := claims["uid"].(string)
		if uid == "" {
			uid, _ = claims["sub"].(string)
		}
		if uid == "" {
			_ = utils.WriteError(w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}

		ctx := context.WithValue(r.Context(), "uid", uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
