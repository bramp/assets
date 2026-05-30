.PHONY: all format analyze test test-ci fix upgrade hooks-install

all: format analyze test

format:
	go fmt ./...
	goimports -w .

analyze:
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
