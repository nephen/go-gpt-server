package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	_ "go-gpt-server/env"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
)

type message_struct struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chat_struct struct {
	Model    string           `json:"model"`
	Stream   bool             `json:"stream"`
	Messages []message_struct `json:"messages"`
}

func HandleChat(w http.ResponseWriter, r *http.Request) {
	var chat chat_struct
	// Read body
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		fmt.Println("ReadAll", err.Error())
	}
	// Unmarshal
	err = json.Unmarshal(b, &chat)
	if err != nil {
		fmt.Println("Unmarshal", err.Error())
	}
	fmt.Println("chat:", chat)

	target := os.Getenv("API_PROXY")
	director := func(req *http.Request) {
		req.URL.Scheme = "http"
		req.URL.Host = target
		req.URL.Path = "/v1/chat/completions"
		req.Host = target
		b, _ := json.Marshal(chat)
		req.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		req.ContentLength = (int64)(len(b))
		req.Header.Set("Authorization", "Bearer "+os.Getenv("CHAT_API_KEY"))
	}
	response := func(res *http.Response) error {
		println("proxy status:", res.Status)
		// 删除重复的响应头
		res.Header.Del("Access-Control-Allow-Origin")
		return nil
	}
	proxy := &httputil.ReverseProxy{Director: director, ModifyResponse: response}
	proxy.ServeHTTP(w, r)
}
