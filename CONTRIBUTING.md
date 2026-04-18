# Contributing to lookout

Thanks for looking. lookout is small on purpose — contributions that keep it
that way are especially welcome.

## Local setup

```bash
git clone https://github.com/alexmchughdev/lookout
cd lookout
./install.sh            # handles Go, Chromium, Ollama, gemma3:12b
```

Or the manual path: `go build ./...` after installing Go 1.22+, Chromium, and
Ollama with `gemma3:12b` pulled.

## Before opening a PR

```bash
go build ./...          # must compile
go vet ./...            # must be clean
gofmt -l .              # must be empty
go test -race ./...     # must pass
lookout validate examples/demo.yaml                 # must succeed
lookout run examples/demo.yaml --retry 1            # must go 8/8 green
```

If you added a new CLI flag, test-spec field, or pre-action type, also update:

- `README.md` (Usage and/or Test fields / Pre-actions tables)
- `cmd/init.go` (the scaffold template)
- `docs/prompts/pdf-to-yaml.md` (the Claude prompt schema)

## Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(runner): --retry N for flaky tests
fix(browser): set desktop viewport
docs: refresh Usage section for current CLI surface
```

Types used in this repo: `feat`, `fix`, `chore`, `docs`, `test`, `ci`, `refactor`.

## What fits lookout

Good fits:

- A new pre-action type (document it in README + init template + prompt).
- A new model provider (add to `internal/vision/judge.go`).
- A CLI flag that improves the interactive or CI experience.
- Bug fixes with a regression test.
- Doc clarifications.

Things we're unlikely to accept:

- Agent-style features where the model drives navigation. Lookout's thesis is
  the opposite — deterministic code drives, the model only observes. If you
  disagree, open an issue first so we can discuss.
- Heavyweight dependencies. Current tree is intentionally small: chromedp,
  cobra, yaml.v3, fatih/color. Adding a full web framework, a DB driver, or
  anything that pulls in a compiler toolchain will get pushback.
- Features that only make sense against one specific app. lookout is a
  framework, not a test suite. Keep behaviour generic.

## Questions

Open an issue with the `question` label, or start a discussion once the
repository has Discussions enabled.
