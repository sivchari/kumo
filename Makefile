.PHONY: build run test test-fuzz test-integration test-helm-e2e clean docker lint lint-fix fmt fmt-diff

BINARY_NAME=kumo
VERSION?=$(shell grep 'const Version' version.go | cut -d'"' -f2)
BUILD_DIR=bin
GOLANGCI_LINT=go tool -modfile tools/go.mod golangci-lint
GOTOOLCHAIN=go1.25.10
export GOTOOLCHAIN

# Build
build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kumo

run:
	go run ./cmd/kumo

# Test
test:
	go test -v -race ./...

test-cover:
	go test -v -race -coverprofile=coverage.out -coverpkg=./... ./...
	go tool cover -html=coverage.out -o coverage.html

test-integration:
	go test -C test -v -tags=integration ./integration/...

test-fuzz:
	@grep -rl '^func Fuzz' internal/ | xargs -I{} dirname {} | sort -u | while read pkg; do \
		grep -oh 'func \(Fuzz[A-Za-z]*\)' "$$pkg"/*_test.go | sed 's/func //' | while read fn; do \
			echo "=== fuzzing $$fn in $$pkg ==="; \
			go test -fuzz="$$fn" -fuzztime=60s "./$$pkg/..." || exit 1; \
		done; \
	done

test-helm-e2e:
	bash test/e2e/helm-e2e.sh

# Lint
lint:
	$(GOLANGCI_LINT) run ./...

lint-fix:
	$(GOLANGCI_LINT) run --fix ./...

fmt:
	$(GOLANGCI_LINT) fmt ./...

fmt-diff:
	$(GOLANGCI_LINT) fmt ./... --diff

# Docker
docker:
	docker build -t kumo:$(VERSION) -f docker/Dockerfile .

docker-run:
	docker run -p 4566:4566 kumo:$(VERSION)

compose-up:
	docker compose up -d

compose-down:
	docker compose down

# Clean
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Tools
tools:
	cd tools && go mod tidy

# Go mod
mod:
	go mod tidy
