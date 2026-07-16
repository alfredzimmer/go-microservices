package tlsconfig

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// writeTestPKI generates a CA and one service identity cert into dir and
// returns the paths (cert, key, ca).
func writeTestPKI(t *testing.T, dir string) (string, string, string) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-service"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	leafKeyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatal(err)
	}

	certPath := filepath.Join(dir, "svc.crt")
	keyPath := filepath.Join(dir, "svc.key")
	caPath := filepath.Join(dir, "ca.crt")
	writePEM(t, certPath, "CERTIFICATE", leafDER)
	writePEM(t, keyPath, "EC PRIVATE KEY", leafKeyDER)
	writePEM(t, caPath, "CERTIFICATE", caDER)
	return certPath, keyPath, caPath
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
}

func setTLSEnv(t *testing.T, cert, key, ca string) {
	t.Helper()
	t.Setenv("TLS_CERT_FILE", cert)
	t.Setenv("TLS_KEY_FILE", key)
	t.Setenv("TLS_CA_FILE", ca)
}

// startServer runs a gRPC server exposing the standard health service with
// the given transport option and returns its address. A successful health
// check proves the handshake and a full RPC round trip.
func startServer(t *testing.T, opt grpc.ServerOption) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer(opt)
	healthpb.RegisterHealthServer(srv, health.NewServer())
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func healthCheck(addr string, dialOpt grpc.DialOption) error {
	conn, err := grpc.NewClient(addr, dialOpt)
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	return err
}

func TestMutualTLSHandshake(t *testing.T) {
	cert, key, ca := writeTestPKI(t, t.TempDir())
	setTLSEnv(t, cert, key, ca)

	serverOpt, err := ServerCredentials()
	if err != nil {
		t.Fatalf("ServerCredentials: %v", err)
	}
	clientOpt, err := ClientCredentials()
	if err != nil {
		t.Fatalf("ClientCredentials: %v", err)
	}

	addr := startServer(t, serverOpt)
	if err := healthCheck(addr, clientOpt); err != nil {
		t.Fatalf("health check over mTLS failed: %v", err)
	}
}

func TestClientWithoutCertRejected(t *testing.T) {
	cert, key, ca := writeTestPKI(t, t.TempDir())
	setTLSEnv(t, cert, key, ca)

	serverOpt, err := ServerCredentials()
	if err != nil {
		t.Fatalf("ServerCredentials: %v", err)
	}
	addr := startServer(t, serverOpt)

	caPEM, err := os.ReadFile(ca)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)
	noIdentity := grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS13,
	}))

	if err := healthCheck(addr, noIdentity); err == nil {
		t.Fatal("client without a certificate got through the mTLS handshake")
	}
}

func TestPlaintextFallbackWhenUnconfigured(t *testing.T) {
	t.Setenv("TLS_CERT_FILE", "")
	t.Setenv("TLS_KEY_FILE", "")
	t.Setenv("TLS_CA_FILE", "")

	if Enabled() {
		t.Fatal("Enabled() should be false with no TLS env")
	}
	serverOpt, err := ServerCredentials()
	if err != nil {
		t.Fatalf("ServerCredentials: %v", err)
	}
	clientOpt, err := ClientCredentials()
	if err != nil {
		t.Fatalf("ClientCredentials: %v", err)
	}

	addr := startServer(t, serverOpt)
	if err := healthCheck(addr, clientOpt); err != nil {
		t.Fatalf("health check over plaintext fallback failed: %v", err)
	}
}
