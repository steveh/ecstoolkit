.PHONY: all
all: generate lint test

.PHONY: test
test:
	go test ./...

.PHONY: generate
generate:
	go generate ./...
	mockery

.PHONY: lint
lint:
	golangci-lint run --fix
