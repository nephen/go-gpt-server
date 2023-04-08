package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
)

const apiKey = "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"

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

	target := "107.148.26.186:5566"
	director := func(req *http.Request) {
		req.URL.Scheme = "http"
		req.URL.Host = target
		req.URL.Path = "/v1/chat/completions"
		req.Host = target
		b, _ := json.Marshal(chat)
		req.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		req.ContentLength = (int64)(len(b))
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	response := func(res *http.Response) error {
		return nil
	}
	proxy := &httputil.ReverseProxy{Director: director, ModifyResponse: response}
	proxy.ServeHTTP(w, r)
}
