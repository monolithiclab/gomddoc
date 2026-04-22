include common-go.mk

ROOT_DIR := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"


build:  ## Build binary
	go build $(LDFLAGS) -o build/gomddoc ./cmd/gomddoc


install:  ## Install gomddoc to $GOBIN (or $GOPATH/bin)
	go install $(LDFLAGS) ./cmd/gomddoc


bench:  ## Run tests and benchmarks
	go test -bench=.


ci:  codefix format lint test  ## Run codefix, format, lint and tests


deploy:  ## Deploy the project
	echo "not implemented" && false


run:  ## Run the application locally
	go run ./cmd/gomddoc serve ../prose-api/docs


test:  ## Run unit tests with coverage and race detection
	go test -race -cover -coverprofile cover.out ./...
	grep -v -E -f .covignore cover.out > cover.out.tmp && mv cover.out.tmp cover.out
	go tool cover -func cover.out


.SILENT: bench build deploy install run test
.PHONY: bench build deploy install run test
