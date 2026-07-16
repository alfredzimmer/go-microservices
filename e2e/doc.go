// Package e2e contains end-to-end tests that run against the full
// docker-compose stack (GraphQL gateway, all three services, their
// databases and Jaeger). They are excluded from normal builds; run them
// with:
//
//	docker compose up -d --build
//	go test -tags e2e ./e2e/...
//
// or simply `make e2e`.
package e2e
