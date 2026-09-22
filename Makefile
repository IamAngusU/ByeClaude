.PHONY: fmt test vet build check

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -trimpath -ldflags "-s -w" -o byeclaude ./cmd/byeclaude

check: fmt vet test build
