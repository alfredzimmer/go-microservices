package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/alfredzimmer/go-microservices/telemetry"
	"github.com/alfredzimmer/go-microservices/tlsconfig"
	"github.com/kelseyhightower/envconfig"
	"github.com/ravilushqa/otelgqlgen"
)

type AppConfig struct {
	AccountURL string `envconfig:"ACCOUNT_SERVICE_URL"`
	CatalogURL string `envconfig:"CATALOG_SERVICE_URL"`
	OrderURL   string `envconfig:"ORDER_SERVICE_URL"`
}

func main() {
	// Exit via run so the deferred span flush still runs.
	os.Exit(run())
}

func run() int {
	ctx := context.Background()
	shutdown, err := telemetry.Bootstrap(ctx, "graphql")
	if err != nil {
		slog.Error("Failed to initialize telemetry", "error", err)
		return 1
	}
	defer shutdown(ctx)

	var cfg AppConfig
	if err := envconfig.Process("", &cfg); err != nil {
		slog.Error("Failed to process config", "error", err)
		return 1
	}

	s, err := NewGraphQLServer(cfg.AccountURL, cfg.CatalogURL, cfg.OrderURL)
	if err != nil {
		slog.Error("Failed to create GraphQL server", "error", err)
		return 1
	}

	srv := handler.NewDefaultServer(s.ToExecutableSchema())
	srv.Use(otelgqlgen.Middleware())

	http.Handle("/graphql", srv)
	http.Handle("/playground", playground.Handler("alfred", "/graphql"))

	if tlsconfig.Enabled() {
		slog.Info("Listening on port 8080 (HTTPS)")
		err = http.ListenAndServeTLS(":8080", tlsconfig.CertFile(), tlsconfig.KeyFile(), nil)
	} else {
		slog.Warn("TLS disabled — serving plaintext HTTP (set TLS_CERT_FILE/TLS_KEY_FILE/TLS_CA_FILE)")
		err = http.ListenAndServe(":8080", nil)
	}
	if err != nil {
		slog.Error("Server stopped", "error", err)
		return 1
	}
	return 0
}
