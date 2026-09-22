# Contributing

Small, reproducible changes are welcome.

Before opening a pull request:

```sh
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
```

For rewrite bugs, include a minimal repository construction script or test that demonstrates the graph shape: branches, merges, tags and the relevant commit messages. Avoid attaching private repository history.

Changes that weaken dry-run behavior, backup creation, verification, or force-with-lease protections need a concrete reason and tests.
