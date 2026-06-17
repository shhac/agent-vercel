BINARY := agent-vercel
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOCACHE ?= $(CURDIR)/.cache/go-build

build:
	GOCACHE=$(GOCACHE) go build -buildvcs=false -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/agent-vercel

test:
	GOCACHE=$(GOCACHE) go test ./... -count=1

test-short:
	GOCACHE=$(GOCACHE) go test ./... -count=1 -short

# Opt-in live tests against the real Vercel API (read-only). Provide a token via
# $AGENT_VERCEL_IT_TOKEN, or a stored credential via $AGENT_VERCEL_IT_AUTH (label)
# + optional $AGENT_VERCEL_IT_SCOPE; they skip if neither is available.
test-integration:
	GOCACHE=$(GOCACHE) go test -tags integration ./... -count=1 -run Live -v

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	@command -v goimports >/dev/null && goimports -w . || echo "goimports not installed (optional; install: go install golang.org/x/tools/cmd/goimports@latest)"

vet:
	GOCACHE=$(GOCACHE) go vet ./...

dev:
	GOCACHE=$(GOCACHE) go run ./cmd/agent-vercel $(ARGS)

clean:
	rm -f $(BINARY)
	rm -rf dist/

.PHONY: build test test-short lint fmt vet dev clean
