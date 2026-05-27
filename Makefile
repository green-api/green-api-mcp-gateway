.PHONY: lint lint-ci test build

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0 run

lint-ci:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.5.0 run

test:
	go test -v -race -count=1 ./... -coverprofile=cover.out

build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(shell git describe --tags --always 2>/dev/null || echo dev)" -o green-api-mcp-gateway ./cmd/server
