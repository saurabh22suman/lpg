GO ?= go
COVERAGE_MIN ?= 50
GOFILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')

.PHONY: fmt fmt-check lint test test-race coverage security ci

fmt:
	@gofmt -w $(GOFILES)

fmt-check:
	@out=$$(gofmt -l $(GOFILES)); \
	if [ -n "$$out" ]; then \
		echo "Unformatted files:"; \
		echo "$$out"; \
		exit 1; \
	fi

lint:
	@golangci-lint run

test:
	@$(GO) test ./...

test-race:
	@$(GO) test -race ./...

coverage:
	@$(GO) test -coverpkg=./... -coverprofile=coverage.out ./...
	@total=$$($(GO) tool cover -func=coverage.out | awk '/^total:/ {gsub("%", "", $$3); print $$3}'); \
	awk -v total="$$total" -v min="$(COVERAGE_MIN)" 'BEGIN { if (total < min) { printf("Coverage %.2f%% is below %.2f%%\n", total, min); exit 1 } }'

security:
	@command -v govulncheck >/dev/null || (echo "govulncheck is required: go install golang.org/x/vuln/cmd/govulncheck@latest" && exit 1)
	@command -v gitleaks >/dev/null || (echo "gitleaks is required: https://github.com/gitleaks/gitleaks" && exit 1)
	@govulncheck ./...
	@gitleaks detect --source . --redact --no-banner --no-git

ci: fmt-check lint test test-race coverage security
