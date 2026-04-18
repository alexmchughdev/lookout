### What this PR does

<!-- One or two sentences. -->

### Why

<!-- The problem you're solving, linked issue if applicable (Fixes #N). -->

### How to test

```bash
# commands a reviewer can run to verify the change
```

### Checklist

- [ ] `go build ./...` passes
- [ ] `go vet ./...` clean
- [ ] `go test -race ./...` passes
- [ ] `gofmt -l .` returns empty
- [ ] For new CLI flags: documented in README Usage section
- [ ] For new test-spec fields: added to `internal/config/config.go`, `cmd/init.go` template, README Test fields table, and `docs/prompts/pdf-to-yaml.md`
- [ ] For behaviour changes: example in `examples/demo.yaml` still passes (`lookout run examples/demo.yaml --retry 1`)
