package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"velox/apps/apigateway/api"
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
	var store *api.DatabaseStore
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		var err error
		store, err = api.OpenDatabaseStore(context.Background(), databaseURL)
		if err != nil {
			slog.Error("open database store failed", "error", err)
			os.Exit(1)
		}
		defer store.Close()
		slog.Info("apigateway using Database reservation store")
	}
	server := api.NewServerWithStore(secret, store)

	if os.Getenv("ORDER_SERVICE_ADDR") != "" {
		server.SetOrderServiceURL("http://" + os.Getenv("ORDER_SERVICE_ADDR") + "/orders")
	}

	slog.Info("apigateway listening", "addr", addr)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
