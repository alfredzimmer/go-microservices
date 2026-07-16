.PHONY: test e2e e2e-up e2e-test e2e-down

# Unit + integration tests (no Docker required).
test:
	go vet ./...
	go test -count=1 ./...

# Full end-to-end run: build and start the compose stack, run the e2e suite
# against it, then tear everything down (even if the tests fail).
e2e:
	docker compose up -d --build
	go test -tags e2e -count=1 -timeout 10m ./e2e/...; status=$$?; \
	docker compose down -v; \
	exit $$status

# Pieces of the above, for keeping the stack alive while iterating.
e2e-up:
	docker compose up -d --build

e2e-test:
	go test -tags e2e -count=1 -timeout 10m -v ./e2e/...

e2e-down:
	docker compose down -v
