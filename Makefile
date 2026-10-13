VERSION ?= 0.1.0
GO ?= go
PNPM ?= pnpm
LDFLAGS = -s -w -X main.version=$(VERSION)

.PHONY: web build release check test install
web:
	$(PNPM) --dir web install --frozen-lockfile
	$(PNPM) --dir web build

build: web
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/webmux ./cmd/webmux

release: web
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/webmux-linux-amd64 ./cmd/webmux
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/webmux-linux-arm64 ./cmd/webmux

check: web
	$(GO) vet ./...
	golangci-lint run ./...

test:
	$(GO) test -race ./...
	$(PNPM) --dir web test

install: build
	install -Dm755 bin/webmux $(HOME)/.local/bin/webmux
