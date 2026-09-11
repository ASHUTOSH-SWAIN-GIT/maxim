.PHONY: all build test test-race test-integration-cli test-integration-db test-integration-docker benchmark-db-100k benchmark-db-1m coverage fmt fmt-check vet tidy tidy-check ci clean

all: ci

build:
	go build -trimpath -o bin/maxim .

test:
	go test ./...

test-race:
	go test -race ./...

test-integration-cli:
	go test -tags=integration -count=1 -v .

test-integration-db:
	./scripts/run-db-integration.sh

test-integration-docker:
	go test -tags=integration -count=1 -v ./internal/docker/...

benchmark-db-100k:
	MAXIM_BENCH_ROWS=100000 ./scripts/run-db-integration.sh

benchmark-db-1m:
	MAXIM_BENCH_ROWS=1000000 ./scripts/run-db-integration.sh

coverage:
	go test -covermode=atomic -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	./scripts/check-coverage.sh coverage.out 15

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

fmt-check:
	test -z "$$(gofmt -l .)"

vet:
	go vet ./...

tidy:
	go mod tidy

tidy-check:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

ci: tidy-check fmt-check vet test-race build

clean:
	rm -rf bin coverage.out dist
