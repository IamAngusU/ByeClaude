.PHONY: fmt test race vet build workflow-lint security installers release-repro verify clean

fmt:
	gofmt -w ./cmd ./internal ./scripts/release-sbom

test:
	go test ./...

vet:
	go vet ./...

race:
	go test -race ./...

build:
	go build -trimpath -buildvcs=false -ldflags "-s -w" -o byeclaude ./cmd/byeclaude

workflow-lint:
	go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7

security:
	go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
	go run github.com/securego/gosec/v2/cmd/gosec@v2.29.0 -exclude-generated ./...

installers:
	bash ./scripts/test-install-shell.sh
	pwsh -NoProfile -File ./scripts/test-install-powershell.ps1

release-repro:
	pwsh -NoProfile -File ./scripts/test-release-reproducibility.ps1

verify: vet test race build workflow-lint security installers release-repro

clean:
	rm -rf byeclaude dist coverage.out release-artifacts
