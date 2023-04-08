package api

import (
	"encoding/json"
	"go-gpt-server/token"
	"net/http"
	"strconv"
	"time"
)

type struct_token struct {
	Token string `json:"token"`
}

func GenToken(w http.ResponseWriter, r *http.Request) {
	if !r.URL.Query().Has("id") || !r.URL.Query().Has("hour") || !r.URL.Query().Has("key") {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	id := r.URL.Query()["id"][0]
	hourStr := r.URL.Query()["hour"][0]
	secretkey := r.URL.Query()["key"][0]

	// 使用 ParseInt() 函数将字符串转换为 int64 类型的整数
	hourData, err := strconv.ParseInt(hourStr, 10, 64)
	// 如果转换失败，处理错误
	if err != nil {
		http.Error(w, "Invalid param time", http.StatusBadRequest)
		return
	}

	tk, err := token.CreateToken(id, time.Duration(hourData*int64(time.Hour)), secretkey)
	if err != nil {
		http.Error(w, "Create token failed", http.StatusServiceUnavailable)
		return
	}
	stk := struct_token{
		Token: tk,
	}
	json.NewEncoder(w).Encode(stk)
}
