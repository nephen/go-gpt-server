package middle

import (
	_ "go-gpt-server/env"
	"go-gpt-server/token"
	"net/http"
	"os"
)

// 鉴权中间件
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tk := r.Header.Get("Authorization")
		id, err := token.ValidateToken(tk, os.Getenv("SECRET_KEY"))
		if err != nil {
			println(err.Error())
			if tk != os.Getenv("SECRET_KEY") {
				println("Invalid token:", tk)
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}
		println("auth pass", id)
		r.Header.Add("UserId", id)
		r.Header.Del("Authorization")
		// 鉴权通过，调用下一个handler
		next.ServeHTTP(w, r)
	})
}
