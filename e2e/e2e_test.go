//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func graphqlURL() string {
	if url := os.Getenv("GRAPHQL_URL"); url != "" {
		return url
	}
	return "http://localhost:8000/graphql"
}

func jaegerURL() string {
	if url := os.Getenv("JAEGER_URL"); url != "" {
		return url
	}
	return "http://localhost:16686"
}

// gql posts a GraphQL query and decodes the data field into out, failing
// the test on transport or GraphQL errors.
func gql(t *testing.T, query string, out any) {
	t.Helper()

	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(graphqlURL(), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("GraphQL request failed: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("invalid GraphQL response: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("GraphQL errors for %s: %+v", query, result.Errors)
	}
	if out != nil {
		if err := json.Unmarshal(result.Data, out); err != nil {
			t.Fatalf("cannot decode GraphQL data: %v", err)
		}
	}
}

// TestMain waits for the compose stack to come up; elasticsearch can take
// a minute, and the services crash-loop until their databases are ready.
func TestMain(m *testing.M) {
	deadline := time.Now().Add(3 * time.Minute)
	for {
		resp, err := http.Post(graphqlURL(), "application/json",
			strings.NewReader(`{"query":"{__typename}"}`))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			fmt.Fprintf(os.Stderr, "GraphQL gateway at %s not ready after 3m (last error: %v)\n", graphqlURL(), err)
			os.Exit(1)
		}
		time.Sleep(2 * time.Second)
	}
	os.Exit(m.Run())
}

// createAccountAndProduct creates a fresh account and a product priced at
// price via the gateway and returns their IDs.
func createAccountAndProduct(t *testing.T, name string, price float64) (accountID, productID string) {
	t.Helper()

	var accountResp struct {
		CreateAccount struct {
			Id string `json:"id"`
		} `json:"createAccount"`
	}
	gql(t, fmt.Sprintf(`mutation E2ECreateAccount { createAccount(account: {name: %q}) { id } }`, name), &accountResp)
	if accountResp.CreateAccount.Id == "" {
		t.Fatal("createAccount returned an empty id")
	}

	var productResp struct {
		CreateProduct struct {
			Id string `json:"id"`
		} `json:"createProduct"`
	}
	gql(t, fmt.Sprintf(`mutation E2ECreateProduct { createProduct(product: {name: "%s-product", description: "made by the e2e suite", price: %g}) { id } }`, name, price), &productResp)
	if productResp.CreateProduct.Id == "" {
		t.Fatal("createProduct returned an empty id")
	}

	return accountResp.CreateAccount.Id, productResp.CreateProduct.Id
}

func TestOrderFlow(t *testing.T) {
	accountID, productID := createAccountAndProduct(t, "e2e-test", 12.5)

	var orderResp struct {
		CreateOrder struct {
			Id         string  `json:"id"`
			TotalPrice float64 `json:"totalPrice"`
		} `json:"createOrder"`
	}
	gql(t, fmt.Sprintf(`mutation E2ECreateOrder { createOrder(order: {accountId: %q, products: [{id: %q, quantity: 2}]}) { id totalPrice } }`, accountID, productID), &orderResp)
	if want := 2 * 12.5; math.Abs(orderResp.CreateOrder.TotalPrice-want) > 1e-9 {
		t.Errorf("expected order total %.2f, got %.2f", want, orderResp.CreateOrder.TotalPrice)
	}

	var accountsResp struct {
		Accounts []struct {
			Name   string `json:"name"`
			Orders []struct {
				Id         string  `json:"id"`
				TotalPrice float64 `json:"totalPrice"`
			} `json:"orders"`
		} `json:"accounts"`
	}
	gql(t, fmt.Sprintf(`query E2EAccountOrders { accounts(id: %q) { name orders { id totalPrice } } }`, accountID), &accountsResp)
	if len(accountsResp.Accounts) != 1 || len(accountsResp.Accounts[0].Orders) == 0 {
		t.Fatalf("expected the account to have at least one order, got %+v", accountsResp.Accounts)
	}
}

// jaegerTrace mirrors the parts of Jaeger's HTTP API response the test needs.
type jaegerTrace struct {
	Spans []struct {
		OperationName string `json:"operationName"`
	} `json:"spans"`
	Processes map[string]struct {
		ServiceName string `json:"serviceName"`
	} `json:"processes"`
}

func (tr jaegerTrace) services() map[string]bool {
	services := map[string]bool{}
	for _, p := range tr.Processes {
		services[p.ServiceName] = true
	}
	return services
}

func (tr jaegerTrace) hasOperation(substr string) bool {
	for _, s := range tr.Spans {
		if strings.Contains(s.OperationName, substr) {
			return true
		}
	}
	return false
}

// TestTraceReachesJaeger verifies the export pipeline the integration tests
// cannot: a createOrder request produces a single trace in Jaeger that
// contains spans from all four services.
func TestTraceReachesJaeger(t *testing.T) {
	// Produce a fresh trace to look for.
	accountID, productID := createAccountAndProduct(t, "e2e-jaeger", 5)
	gql(t, fmt.Sprintf(`mutation E2EJaegerOrder { createOrder(order: {accountId: %q, products: [{id: %q, quantity: 1}]}) { id totalPrice } }`, accountID, productID), nil)

	// The batch exporter flushes every few seconds; poll Jaeger for the trace.
	wantServices := []string{"graphql", "order", "account", "catalog"}
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		trace, err := findTrace(wantServices, "E2EJaegerOrder")
		if err == nil && trace != nil {
			return
		}
		lastErr = err
		time.Sleep(3 * time.Second)
	}
	t.Fatalf("no trace containing services %v and operation E2EJaegerOrder found in Jaeger within 60s (last error: %v)", wantServices, lastErr)
}

// findTrace queries Jaeger for recent graphql traces and returns one that
// spans all wanted services and contains the given operation name.
func findTrace(wantServices []string, operation string) (*jaegerTrace, error) {
	resp, err := http.Get(jaegerURL() + "/api/traces?service=graphql&lookback=5m&limit=50")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jaeger API returned %s", resp.Status)
	}

	var result struct {
		Data []jaegerTrace `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

search:
	for _, tr := range result.Data {
		if !tr.hasOperation(operation) {
			continue
		}
		services := tr.services()
		for _, want := range wantServices {
			if !services[want] {
				continue search
			}
		}
		return &tr, nil
	}
	return nil, fmt.Errorf("%d recent traces checked, none matched", len(result.Data))
}
