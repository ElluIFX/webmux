VERSION ?= 0.1.0
GO ?= go
PNPM ?= pnpm
GOFMT ?= gofmt
LDFLAGS = -s -w -X main.version=$(VERSION)

.PHONY: web build release check test format format-check install
web:
	$(PNPM) --dir web install --frozen-lockfile
	$(PNPM) --dir web build

build: web
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/webmux ./cmd/webmux

release: web
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/webmux-linux-amd64 ./cmd/webmux
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/webmux-linux-arm64 ./cmd/webmux

check: web format-check
	$(GO) vet ./...
	golangci-lint run ./...

test:
	$(GO) test -race ./...
	$(PNPM) --dir web test

format:
	$(GOFMT) -w cmd internal web/embed.go
	$(PNPM) --dir web exec prettier --write --ignore-path ../.prettierignore ../README.md ../WebMux.md ../.prettierrc.json .

format-check:
	@files="$$($(GOFMT) -l cmd internal web/embed.go)"; \
		if [ -n "$$files" ]; then printf 'Unformatted Go files:\n%s\n' "$$files"; exit 1; fi
	$(PNPM) --dir web exec prettier --check --ignore-path ../.prettierignore ../README.md ../WebMux.md ../.prettierrc.json .

install: build
	install -Dm755 bin/webmux $(HOME)/.local/bin/webmux
