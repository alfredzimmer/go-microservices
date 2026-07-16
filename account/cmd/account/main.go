package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/alfredzimmer/go-microservices/account"
	"github.com/alfredzimmer/go-microservices/telemetry"
	"github.com/kelseyhightower/envconfig"
	"github.com/tinrab/retry"
)

type Config struct {
	DatabaseURL string `envconfig:"DATABASE_URL"`
}

func main() {
	// Exit via run so deferred cleanup (span flush, DB close) still runs.
	os.Exit(run())
}

func run() int {
	ctx := context.Background()
	shutdown, err := telemetry.Bootstrap(ctx, "account")
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

	var r account.Repository
	retry.ForeverSleep(2*time.Second, func(_ int) (err error) {
		r, err = account.NewPostgresRepository(cfg.DatabaseURL)

		if err != nil {
			slog.Error("Failed to connect to database", "error", err)
		}
		return
	})
	defer r.Close()

	slog.Info("Listening on port 8080")
	s := account.NewService(r)
	if err := account.ListenGRPC(s, 8080); err != nil {
		slog.Error("Server stopped", "error", err)
		return 1
	}
	return 0
}
