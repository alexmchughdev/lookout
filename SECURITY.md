# Security policy

## Reporting a vulnerability

Please **do not file a public issue** for security-sensitive problems.

Instead, use GitHub's private vulnerability reporting:

https://github.com/alexmchughdev/lookout/security/advisories/new

You'll typically hear back within a few days. Once a fix is ready we'll
coordinate a disclosure timeline with you and credit you in the release notes
if you want.

## What counts as a security issue

- Anything that leaks credentials or session cookies from a running lookout
  process — `.lookout/session.json` exposure, cookies written to disk with
  loose permissions, cookies bleeding into log output, etc.
- Path traversal or arbitrary file reads via crafted spec files.
- Code execution from a malicious YAML spec or PDF.
- Network requests to endpoints other than the configured app / vision
  provider.
- Anything that would let a malicious spec exfiltrate data from the host
  running `lookout`.

## What does NOT count

- Issues in Chromium, Ollama, or other upstream tools — report those upstream.
- Your own test spec being stolen from your laptop by someone who already has
  shell access (that's outside the threat model).
- Local session files being readable by your own user (they're `chmod 0600`
  by design — anyone with your user's permissions can read them, which
  matches how `~/.aws/credentials`, `~/.ssh/`, and `~/.kube/config` work).

## Scope

lookout is a QA tool you run against your own apps. The typical threat model:

- You trust the YAML spec you wrote.
- You trust the target app you're testing.
- You don't trust the *screenshots* (they can't execute anything — they're
  passed to a vision model as binary PNG data).
- You don't trust stray YAML you found on the internet — open an issue if
  you find a way for a malicious spec to misbehave.
