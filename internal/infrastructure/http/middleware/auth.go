package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Auth retorna un middleware que valida el Bearer token del header Authorization.
// Si token está vacío, el middleware no aplica ninguna restricción.
func Auth(token string, next http.Handler) http.Handler {
	if token == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)

		if len(parts) != 2 || parts[0] != "Bearer" || parts[1] != token {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"}) //nolint:errcheck
			return
		}

		next.ServeHTTP(w, r)
	})
}
