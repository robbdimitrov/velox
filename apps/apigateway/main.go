package main

import (
	"context"
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
	var store *internal.PostgresStore
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		var err error
		store, err = internal.OpenPostgresStore(context.Background(), databaseURL)
		if err != nil {
			log.Fatalf("open postgres store: %v", err)
		}
		defer store.Close()
		log.Print("apigateway using PostgreSQL reservation store")
	}
	server := internal.NewServerWithStore(secret, store)
	log.Printf("apigateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, server.Routes()))
}
