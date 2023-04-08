package middle

import "net/http"

// 中间件处理函数
func HandleOptions(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 如果是OPTIONS请求，返回允许的HTTP头
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// 如果不是OPTIONS请求，调用处理请求的handler
		handler.ServeHTTP(w, r)
	})
}
