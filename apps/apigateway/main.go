package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
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

	// REDIS_ADDR is the deployment's actual configured name (velox-service-config);
	// REDIS_URL is kept as an override for other contexts (local dev, tests).
	cacheURL := os.Getenv("REDIS_URL")
	if cacheURL == "" {
		cacheURL = os.Getenv("REDIS_ADDR")
	}
	if cacheURL == "" {
		cacheURL = "localhost:6379"
	}
	cacheClient := redis.NewClient(&redis.Options{
		Addr:            cacheURL,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	})

	server := api.NewServerWithStore(secret, store, cacheClient)
	server.SetTokenIssuerAudience(os.Getenv("JWT_ISSUER"), os.Getenv("JWT_AUDIENCE"))

	if os.Getenv("ORDER_SERVICE_ADDR") != "" {
		server.SetOrderServiceURL("http://" + os.Getenv("ORDER_SERVICE_ADDR") + "/orders")
	}

	slog.Info("apigateway listening", "addr", addr)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
