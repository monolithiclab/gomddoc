include common-go.mk

ROOT_DIR := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))


build:  ## Build binary
	go build -o build/gomddoc ./cmd/gomddoc


bench:  ## Run tests and benchmarks
	go test -bench=.


deploy:  ## Deploy the project
	echo "not implemented" && false


run:  ## Run the application locally
	go run ./cmd/gomddoc -d ../prose-api/docs


test:  ## Run unit tests with coverage
	go test -cover -coverprofile cover.out ./...
	go tool cover -func cover.out


.SILENT: bench build deploy run test
.PHONY: bench build deploy run test
