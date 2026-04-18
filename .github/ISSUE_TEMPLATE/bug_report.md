---
name: Bug report
about: Something broke. Help us reproduce it.
title: ''
labels: bug
assignees: ''
---

### What happened

<!-- One or two sentences. -->

### What you expected

<!-- What should have happened instead. -->

### Reproduction

Minimal spec (redact anything private):

```yaml
# paste the smallest lookout.yaml that reproduces the issue
```

Command you ran:

```bash
lookout run tests.yaml --headed --retry 1
```

### Terminal output

```
paste the full output here
```

### Environment

- lookout version: <!-- output of `lookout --version` -->
- OS: <!-- e.g. Ubuntu 22.04, macOS 14.4, Arch -->
- Go version: <!-- `go version` if you built from source -->
- Browser: <!-- output of `which chromium` or similar -->
- Vision provider: <!-- ollama gemma3:12b, anthropic claude-sonnet-4-6, openai gpt-5.4 -->

### Screenshots

<!-- If relevant: attach the HTML report, a terminal screenshot, or individual screenshots from reports/. -->
