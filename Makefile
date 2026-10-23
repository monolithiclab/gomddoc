include common.mk
include common.go.mk

CI_EXTRA = test-ui

# Scoped to what ships, for the same reason .covignore excludes docs/skills/: nothing under docs/ reaches the binary,
# but govulncheck traces its imports all the same, so an image/png CVE would fail the gate over a tool that is never
# distributed. image/png enters the graph solely via favicon-check.go.
VULNCHECK_PACKAGES = ./cmd/... ./internal/...
COVERAGE_MIN = 87

.PHONY: run
run: ## Serve the lorem-ipsum testsite locally (preview server, writes nothing)
	@go run $(MAIN) serve testsite

# Front-end tests (D48) run only when their inputs change: the shared scripts and styles, the default theme's
# templates, the locales and web/ itself. web/node_modules and the Playwright output never enter the hash, so a
# fresh clone and a warm checkout hash alike.
UI_INPUTS := $(sort $(wildcard cmd/gomddoc/assets/shared/*.mjs cmd/gomddoc/assets/shared/*.css \
	cmd/gomddoc/assets/themes/default/layouts/*.tmpl cmd/gomddoc/assets/themes/default/partials/*.tmpl \
	cmd/gomddoc/assets/locales/*.yml) \
	$(shell find web \( -name node_modules -o -name test-results -o -name playwright-report \) -prune -o -type f -print))

.PHONY: test-ui
test-ui: ## Front-end tests (Vitest, Playwright), skipped when their inputs are unchanged
	@hash=$$(shasum -a 256 $(UI_INPUTS) | shasum -a 256 | cut -d' ' -f1); \
	if [ "$$hash" = "$$(cat .cache/test-ui.sha 2>/dev/null)" ]; then \
		echo "test-ui: front-end unchanged, skipped"; exit 0; \
	fi; \
	command -v npm >/dev/null || { echo "test-ui: front-end changed and needs Node >= 22 (brew install node)"; exit 1; }; \
	if [ web/package-lock.json -nt web/node_modules/.package-lock.json ]; then \
		npm ci --prefix web --no-audit --no-fund || exit 1; \
	fi; \
	npm --prefix web run --silent unit || exit 1; \
	mkdir -p .cache && echo "$$hash" > .cache/test-ui.sha
