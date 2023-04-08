package middle

import (
	"go-gpt-server/token"
	"net/http"
)

const chat_code = "dyhlyb"

// 鉴权中间件
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tk := r.Header.Get("Authorization")
		id, err := token.ValidateToken(tk, chat_code)
		if err != nil {
			println(err.Error())
			if tk != chat_code {
				println("Invalid token: ", tk)
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}
		println("auth pass ", id)
		r.Header.Add("UserId", id)
		r.Header.Del("Authorization")
		// 鉴权通过，调用下一个handler
		next.ServeHTTP(w, r)
	})
}
