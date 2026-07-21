.PHONY: build dev test test-integration lint

build:
	go build ./...

dev:
	go run cmd/server/main.go

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

lint:
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		go vet ./...; \
	fi
