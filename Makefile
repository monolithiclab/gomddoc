include common.mk
include common.go.mk

# Scoped to what ships, for the same reason .covignore excludes docs/skills/: nothing under docs/ reaches the binary,
# but govulncheck traces its imports all the same, so an image/png CVE would fail the gate over a tool that is never
# distributed. image/png enters the graph solely via favicon-check.go.
VULNCHECK_PACKAGES = ./cmd/... ./internal/...
COVERAGE_MIN = 87

.PHONY: run
run: ## Serve the lorem-ipsum testsite locally (preview server, writes nothing)
	@go run $(MAIN) serve testsite
