package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/alfredzimmer/go-microservices/order"
	"github.com/alfredzimmer/go-microservices/telemetry"
	"github.com/kelseyhightower/envconfig"
	"github.com/tinrab/retry"
)

type Config struct {
	DatabaseURL string `envconfig:"DATABASE_URL"`
	AccountURL  string `envconfig:"ACCOUNT_SERVICE_URL"`
	CatalogURL  string `envconfig:"CATALOG_SERVICE_URL"`
}

func main() {
	// Exit via run so deferred cleanup (span flush, DB close) still runs.
	os.Exit(run())
}

func run() int {
	ctx := context.Background()
	shutdown, err := telemetry.Bootstrap(ctx, "order")
	if err != nil {
		slog.Error("Failed to initialize telemetry", "error", err)
		return 1
	}
	defer shutdown(ctx)

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		slog.Error("Failed to process config", "error", err)
		return 1
	}

	var r order.Repository
	retry.ForeverSleep(2*time.Second, func(_ int) (err error) {
		r, err = order.NewPostgresRepository(cfg.DatabaseURL)
		if err != nil {
			slog.Error("Failed to connect to database", "error", err)
		}
		return
	})
	defer r.Close()

	slog.Info("Listening on port 8080")
	s := order.NewService(r)
	if err := order.ListenGRPC(s, cfg.AccountURL, cfg.CatalogURL, 8080); err != nil {
		slog.Error("Server stopped", "error", err)
		return 1
	}
	return 0
}
