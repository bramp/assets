.PHONY: all format lint analyze test test-ci fix upgrade hooks-install

all: format analyze test

format:
	go fmt ./...
	goimports -w .

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found; install from https://golangci-lint.run/welcome/install/"; exit 1; }
	@golangci-lint --version | grep -q "version 2\." || { echo "golangci-lint v2 is required by .golangci.yml"; exit 1; }
	golangci-lint run --config .golangci.yml ./...

analyze:
	$(MAKE) lint
	go vet ./...
	staticcheck ./...

test:
	go test ./...

test-ci:
	go test -v ./...

fix:
	go fmt ./...
	go fix ./...

upgrade:
	go mod tidy
	go get -u ./...
	go mod tidy

hooks-install:
	@command -v pre-commit >/dev/null 2>&1 || { echo "pre-commit not found; install it first"; exit 1; }
	pre-commit install --hook-type pre-commit
	@echo "pre-commit hook installed"
