package main

import (
	"net/http"

	"go-gpt-server/api"
	"go-gpt-server/middle"
)

func main() {
	http.Handle("/event", middle.EnableCors(middle.HandleOptions(http.HandlerFunc(api.HandleSSE))))
	http.Handle("/token", http.HandlerFunc(api.GenToken))
	http.Handle("/chat", middle.EnableCors(middle.HandleOptions(middle.AuthMiddleware(http.HandlerFunc(api.HandleChat)))))
	http.Handle("/conv", middle.EnableCors(middle.HandleOptions(middle.AuthMiddleware(http.HandlerFunc(api.HandleConv)))))
	println("Welcome to the gpt server")
	http.ListenAndServe(":8000", nil)
}
