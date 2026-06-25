package main

import (
	"log"
	"net/http"
	"os"

	"velox/apps/apigateway/internal"
)

func main() {
	secret := os.Getenv("VELOX_SESSION_SECRET")
	if secret == "" {
		secret = "dev-only-change-me"
	}
	addr := os.Getenv("VELOX_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := internal.NewServer(secret)
	log.Printf("apigateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, server.Routes()))
}
