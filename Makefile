.PHONY: build test lint release-snapshot

build:
	go build -o raku ./cmd/raku

test:
	go test ./...

lint:
	golangci-lint run

release-snapshot:
	goreleaser release --snapshot --clean
