package middle

import "net/http"

// 启用CORS支持的中间件
func EnableCors(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 添加HTTP头，启用CORS支持
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// 调用处理请求的handler
		handler.ServeHTTP(w, r)
	})
}
