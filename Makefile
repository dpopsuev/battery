.PHONY: test lint lint-new build clean

# Default: test + lint
test:
	go test -race -count=1 ./...

# Lint
lint:
	golangci-lint run ./...

# Lint only new changes (used by pre-commit hook)
lint-new:
	golangci-lint run --new-from-rev=HEAD ./...

# Build
build:
	go build ./...

# Clean test caches
clean:
	go clean -testcache
