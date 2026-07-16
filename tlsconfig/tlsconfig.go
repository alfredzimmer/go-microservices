// Package tlsconfig builds the gRPC transport credentials used for mutual TLS
// between services. Every service holds an identity certificate signed by the
// shared local CA and presents it on both sides of a connection: as a server
// it requires and verifies the client's certificate, and as a client it
// verifies the server against the same CA.
//
// Configuration comes from three environment variables, set by
// docker-compose for every service:
//
//	TLS_CERT_FILE  path to the service's certificate
//	TLS_KEY_FILE   path to the service's private key
//	TLS_CA_FILE    path to the CA certificate used to verify peers
//
// When they are unset (unit and integration tests running in-process), both
// helpers fall back to plaintext and log a warning.
package tlsconfig

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Enabled reports whether TLS is configured via the environment.
func Enabled() bool {
	return os.Getenv("TLS_CERT_FILE") != ""
}

// CertFile and KeyFile expose the configured paths for servers that speak
// plain HTTPS rather than gRPC (the GraphQL gateway).
func CertFile() string { return os.Getenv("TLS_CERT_FILE") }
func KeyFile() string  { return os.Getenv("TLS_KEY_FILE") }

// ServerCredentials returns the grpc.ServerOption enforcing mutual TLS, or a
// no-op option with a warning when TLS is not configured.
func ServerCredentials() (grpc.ServerOption, error) {
	if !Enabled() {
		slog.Warn("TLS disabled — serving plaintext gRPC (set TLS_CERT_FILE/TLS_KEY_FILE/TLS_CA_FILE)")
		return grpc.Creds(insecure.NewCredentials()), nil
	}
	cert, pool, err := loadIdentity()
	if err != nil {
		return nil, err
	}
	return grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	})), nil
}

// ClientCredentials returns the grpc.DialOption presenting this service's
// identity and verifying the server against the CA, or plaintext with a
// warning when TLS is not configured.
func ClientCredentials() (grpc.DialOption, error) {
	if !Enabled() {
		slog.Warn("TLS disabled — dialing plaintext gRPC (set TLS_CERT_FILE/TLS_KEY_FILE/TLS_CA_FILE)")
		return grpc.WithTransportCredentials(insecure.NewCredentials()), nil
	}
	cert, pool, err := loadIdentity()
	if err != nil {
		return nil, err
	}
	return grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS13,
	})), nil
}

func loadIdentity() (tls.Certificate, *x509.CertPool, error) {
	cert, err := tls.LoadX509KeyPair(os.Getenv("TLS_CERT_FILE"), os.Getenv("TLS_KEY_FILE"))
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("loading TLS keypair: %w", err)
	}
	caPEM, err := os.ReadFile(os.Getenv("TLS_CA_FILE"))
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("reading CA file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return tls.Certificate{}, nil, fmt.Errorf("no certificates parsed from %s", os.Getenv("TLS_CA_FILE"))
	}
	return cert, pool, nil
}
