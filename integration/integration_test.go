// Package integration starts the real account, catalog and order gRPC
// servers in-process (with in-memory storage) and verifies the behavior
// that unit tests cannot: that a call fans out across services, that all
// resulting spans belong to a single trace, and that error logs carry the
// trace ID of the request that caused them.
//
// These tests need no Docker and run as part of `go test ./...`.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alfredzimmer/go-microservices/account"
	"github.com/alfredzimmer/go-microservices/catalog"
	"github.com/alfredzimmer/go-microservices/order"
	"github.com/alfredzimmer/go-microservices/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const (
	knownAccountID = "account-1"
	knownProductID = "product-1"
	productPrice   = 9.99
)

var (
	recorder    *tracetest.SpanRecorder
	logs        *safeBuffer
	orderClient *order.Client
)

// safeBuffer guards the shared log buffer against concurrent writes from
// the in-process servers.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) Lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Split(b.buf.String(), "\n")
}

type fakeAccountService struct{}

func (fakeAccountService) PostAccount(ctx context.Context, name string) (*account.Account, error) {
	return &account.Account{Id: knownAccountID, Name: name}, nil
}

func (fakeAccountService) GetAccount(ctx context.Context, id string) (*account.Account, error) {
	if id != knownAccountID {
		return nil, errors.New("account not found")
	}
	return &account.Account{Id: id, Name: "Test Account"}, nil
}

func (fakeAccountService) GetAccounts(ctx context.Context, skip uint64, take uint64) ([]account.Account, error) {
	return []account.Account{{Id: knownAccountID, Name: "Test Account"}}, nil
}

type fakeCatalogService struct{}

func (fakeCatalogService) product() catalog.Product {
	return catalog.Product{Id: knownProductID, Name: "Test Product", Description: "A product", Price: productPrice}
}

func (f fakeCatalogService) PostProduct(ctx context.Context, name string, description string, price float64) (*catalog.Product, error) {
	p := f.product()
	return &p, nil
}

func (f fakeCatalogService) GetProductById(ctx context.Context, id string) (*catalog.Product, error) {
	if id != knownProductID {
		return nil, errors.New("product not found")
	}
	p := f.product()
	return &p, nil
}

func (f fakeCatalogService) GetProducts(ctx context.Context, skip uint64, take uint64) ([]catalog.Product, error) {
	return []catalog.Product{f.product()}, nil
}

func (f fakeCatalogService) GetProductsByIds(ctx context.Context, ids []string) ([]catalog.Product, error) {
	products := []catalog.Product{}
	for _, id := range ids {
		if id == knownProductID {
			products = append(products, f.product())
		}
	}
	return products, nil
}

func (f fakeCatalogService) SearchProducts(ctx context.Context, query string, skip uint64, take uint64) ([]catalog.Product, error) {
	return []catalog.Product{f.product()}, nil
}

// inMemoryOrderRepository backs the real order service with a slice, so the
// order code under test is the production service and gRPC server.
type inMemoryOrderRepository struct {
	mu     sync.Mutex
	orders []order.Order
	byKey  map[string]order.Order
}

func (r *inMemoryOrderRepository) Close() {}

func (r *inMemoryOrderRepository) PutOrder(ctx context.Context, o order.Order, idempotencyKey string) (order.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byKey == nil {
		r.byKey = map[string]order.Order{}
	}
	if existing, ok := r.byKey[idempotencyKey]; ok {
		return existing, nil
	}
	r.orders = append(r.orders, o)
	r.byKey[idempotencyKey] = o
	return o, nil
}

func (r *inMemoryOrderRepository) GetOrdersForAccount(ctx context.Context, accountId string) ([]order.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	orders := []order.Order{}
	for _, o := range r.orders {
		if o.AccountId == accountId {
			orders = append(orders, o)
		}
	}
	return orders, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// startServer runs a ListenGRPC-style function on a free port and waits for
// the port to accept connections.
func startServer(listen func(port int) error) (string, error) {
	port, err := freePort()
	if err != nil {
		return "", err
	}
	go func() {
		if err := listen(port); err != nil {
			slog.Error("test server stopped", "error", err)
		}
	}()

	addr := fmt.Sprintf("localhost:%d", port)
	for i := 0; i < 50; i++ {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			return addr, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", fmt.Errorf("server on %s never became reachable", addr)
}

func TestMain(m *testing.M) {
	// Record spans in memory instead of exporting over OTLP, and set the
	// propagator the services normally get from telemetry.Init.
	recorder = tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	otel.SetTextMapPropagator(propagation.TraceContext{})

	logs = &safeBuffer{}
	slog.SetDefault(telemetry.NewLogger(logs, "integration"))

	accountAddr, err := startServer(func(port int) error {
		return account.ListenGRPC(fakeAccountService{}, port)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	catalogAddr, err := startServer(func(port int) error {
		return catalog.ListenGRPC(fakeCatalogService{}, port)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	orderAddr, err := startServer(func(port int) error {
		return order.ListenGRPC(order.NewService(&inMemoryOrderRepository{}), accountAddr, catalogAddr, port)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	orderClient, err = order.NewClient(orderAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	code := m.Run()
	orderClient.Close()
	os.Exit(code)
}

// runInSpan invokes fn inside a root span and returns the trace ID, so the
// spans and logs produced by fn can be filtered out of the shared recorder.
func runInSpan(name string, fn func(ctx context.Context)) trace.TraceID {
	ctx, span := otel.Tracer("integration").Start(context.Background(), name)
	defer span.End()
	fn(ctx)
	return span.SpanContext().TraceID()
}

func spanNamesForTrace(traceID trace.TraceID) map[string]bool {
	names := map[string]bool{}
	for _, s := range recorder.Ended() {
		if s.SpanContext().TraceID() == traceID {
			names[s.Name()] = true
		}
	}
	return names
}

func logRecordsForTrace(traceID trace.TraceID) []map[string]any {
	records := []map[string]any{}
	for _, line := range logs.Lines() {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if record["trace_id"] == traceID.String() {
			records = append(records, record)
		}
	}
	return records
}

// TestPostOrderPropagatesOneTrace places an order through the real order
// gRPC server, which fans out to the account and catalog servers, and
// verifies both the business result and that every hop recorded its spans
// under the same trace.
func TestPostOrderPropagatesOneTrace(t *testing.T) {
	var placed *order.Order
	var err error
	traceID := runInSpan("post-order", func(ctx context.Context) {
		placed, err = orderClient.PostOrder(ctx, knownAccountID, []order.OrderedProduct{
			{Id: knownProductID, Quantity: 2},
		}, "")
	})

	if err != nil {
		t.Fatalf("PostOrder failed: %v", err)
	}
	if want := 2 * productPrice; placed.TotalPrice != want {
		t.Errorf("expected total price %.2f, got %.2f", want, placed.TotalPrice)
	}

	names := spanNamesForTrace(traceID)
	for _, want := range []string{
		"pb.OrderService/PostOrder",
		"pb.AccountService/GetAccount",
		"pb.CatalogService/GetProducts",
	} {
		if !names[want] {
			t.Errorf("expected a span named %q in the trace, got %v", want, names)
		}
	}
}

// TestPostOrderIdempotent verifies that two PostOrder calls carrying the same
// idempotency key place the order once: the second call returns the order
// created by the first instead of a new one.
func TestPostOrderIdempotent(t *testing.T) {
	key := "integration-idempotency-key"
	products := []order.OrderedProduct{{Id: knownProductID, Quantity: 3}}

	first, err := orderClient.PostOrder(context.Background(), knownAccountID, products, key)
	if err != nil {
		t.Fatalf("first PostOrder failed: %v", err)
	}
	second, err := orderClient.PostOrder(context.Background(), knownAccountID, products, key)
	if err != nil {
		t.Fatalf("second PostOrder (retry) failed: %v", err)
	}

	if first.Id != second.Id {
		t.Errorf("retry with the same key should return order id %q, got %q", first.Id, second.Id)
	}
	if first.TotalPrice != second.TotalPrice {
		t.Errorf("retry should return the same total %.2f, got %.2f", first.TotalPrice, second.TotalPrice)
	}
}

// TestFailedOrderLogsWithTraceID triggers a failure inside the order
// service and verifies the error is logged as JSON carrying the trace ID of
// the request that caused it — the property that lets a log line be looked
// up in Jaeger.
func TestFailedOrderLogsWithTraceID(t *testing.T) {
	var err error
	traceID := runInSpan("failing-order", func(ctx context.Context) {
		_, err = orderClient.PostOrder(ctx, "no-such-account", []order.OrderedProduct{
			{Id: knownProductID, Quantity: 1},
		}, "")
	})

	if err == nil {
		t.Fatal("expected PostOrder with unknown account to fail")
	}
	if !strings.Contains(err.Error(), "account not found") {
		t.Errorf("expected 'account not found' error, got: %v", err)
	}

	for _, record := range logRecordsForTrace(traceID) {
		if record["msg"] == "Error getting account" && record["accountId"] == "no-such-account" {
			return
		}
	}
	t.Errorf("no 'Error getting account' log record found with trace_id %s", traceID)
}

// TestGetOrdersForAccountPropagatesOneTrace covers the read path: order
// service -> repository plus the catalog fan-out used to decorate orders.
func TestGetOrdersForAccountPropagatesOneTrace(t *testing.T) {
	var err error
	traceID := runInSpan("get-orders", func(ctx context.Context) {
		if _, err = orderClient.PostOrder(ctx, knownAccountID, []order.OrderedProduct{{Id: knownProductID, Quantity: 1}}, ""); err != nil {
			return
		}
		var orders []order.Order
		orders, err = orderClient.GetOrdersForAccount(ctx, knownAccountID)
		if err == nil && len(orders) == 0 {
			err = errors.New("expected at least one order for account")
		}
	})
	if err != nil {
		t.Fatalf("order read path failed: %v", err)
	}

	names := spanNamesForTrace(traceID)
	for _, want := range []string{
		"pb.OrderService/PostOrder",
		"pb.OrderService/GetOrdersForAccount",
		"pb.CatalogService/GetProducts",
	} {
		if !names[want] {
			t.Errorf("expected a span named %q in the trace, got %v", want, names)
		}
	}
}
