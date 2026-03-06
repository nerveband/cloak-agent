VERSION ?= $(shell grep 'var Version' cmd/root.go | head -1 | sed 's/.*"\(.*\)"/\1/')
LDFLAGS := -ldflags "-X github.com/nerveband/cloak-agent/cmd.Version=$(VERSION)"

.PHONY: build test install clean daemon-build daemon-test cli-build cli-test

# Build everything
build: daemon-build cli-build
	@echo "Build complete: ./cloak-agent (v$(VERSION))"

# Run all tests
test: daemon-test cli-test

# Install globally
install: build
	./scripts/install.sh

# Clean build artifacts
clean:
	rm -f cloak-agent
	rm -rf daemon/dist

# Daemon targets
daemon-build:
	cd daemon && npm run build

daemon-test:
	cd daemon && npm test

# CLI targets
cli-build:
	go build $(LDFLAGS) -o cloak-agent .

cli-test:
	go test ./cmd/... -v

# Development
dev:
	@echo "Starting daemon in watch mode..."
	cd daemon && npm run dev &
	@echo "Run 'go build -o cloak-agent .' to rebuild CLI"
