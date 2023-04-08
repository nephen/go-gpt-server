package main

import (
	"log"
	"net/http"

	"go-gpt-server/api"
	"go-gpt-server/middle"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	http.Handle("/event", middle.EnableCors(middle.HandleOptions(http.HandlerFunc(api.HandleSSE))))
	http.Handle("/token", http.HandlerFunc(api.GenToken))
	http.Handle("/chat", middle.EnableCors(middle.HandleOptions(middle.AuthMiddleware(http.HandlerFunc(api.HandleChat)))))
	http.Handle("/conv", middle.EnableCors(middle.HandleOptions(middle.AuthMiddleware(http.HandlerFunc(api.HandleConv)))))
	println("Welcome to the gpt server")
	http.ListenAndServe(":8000", nil)
}
