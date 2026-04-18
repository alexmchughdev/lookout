<p align="center">
  <img src="docs/banner.webp" alt="lookout" width="640">
</p>

<p align="center">
  <em>visual QA · local-first · single binary</em>
</p>

<p align="center">
  <a href="https://github.com/alexmchughdev/lookout/actions/workflows/ci.yml"><img src="https://github.com/alexmchughdev/lookout/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="License: MIT"></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/go-1.22%2B-00ADD8?logo=go" alt="Go 1.22+"></a>
  <a href="https://github.com/alexmchughdev/lookout/releases"><img src="https://img.shields.io/github/v/release/alexmchughdev/lookout" alt="Release"></a>
</p>

**lookout** automates E2E visual QA. chromedp navigates your app deterministically. A vision model looks at each screenshot and returns a Pass/Fail verdict. No agent loops. No cloud required by default. Single binary.

## Hardware requirements

lookout needs Chromium (to drive the browser) and a vision model (to judge
screenshots). The model is the only expensive dependency — you can run it
locally on a GPU, locally on CPU (slower), or via a hosted API (no GPU
needed at all).

| Setup | GPU | RAM | Per-test latency | Notes |
|-------|-----|-----|------------------|-------|
| Ollama + `gemma3:12b` (default) | ~8 GB VRAM | 16 GB | 0.5–1.5s | Best local accuracy / speed tradeoff |
| Ollama + `qwen2.5vl:7b` | ~5 GB VRAM | 12 GB | 0.4–1s | Faster, less memory |
| Ollama + `llama3.2-vision:11b` | ~7 GB VRAM | 16 GB | 0.5–1.5s | Alternative |
| Ollama on CPU | — | 32 GB | 15–60s | Works, but slow — fine for a handful of tests |
| Anthropic API (`claude-sonnet-4-6`) | — | 4 GB | 1–3s (network) | Highest accuracy, per-token cost |
| OpenAI API (`gpt-5.4`) | — | 4 GB | 1–3s (network) | Strong vision, per-token cost |

Also required on the host regardless of model: Chromium (~1 GB RAM during runs)
and Go 1.22+ if you're building from source.

If you don't have a GPU and don't want to wait for CPU inference, skip
straight to the API-key section below.

## Install

Three options, pick whichever matches your setup.

### Option 1 — Docker (no OS dependencies)

If you have Docker, this is the simplest path. Nothing else to install:

```bash
# Run tests against any target app, using a hosted vision API
docker run --rm \
  -v "$PWD:/work" \
  -e LOOKOUT_API_KEY \
  ghcr.io/alexmchughdev/lookout:latest \
  run tests.yaml --provider anthropic --model claude-sonnet-4-6 \
                 --no-gpu-monitor --no-open
```

The image bundles Chromium and the lookout binary (~380 MB). Your `tests.yaml`
and the generated `reports/` live on your host via the volume mount. Works
identically on Linux, macOS, and Windows — Docker hides the differences.

For local Ollama without installing it on the host, use the included
[`docker-compose.yml`](docker-compose.yml) which spins up an Ollama sidecar:

```bash
docker compose up -d ollama
docker compose exec ollama ollama pull gemma3:12b
docker compose run --rm lookout run examples/demo.yaml --no-gpu-monitor --no-open
```

### Option 2 — `install.sh` (native install)

If you want `lookout` on your `PATH`, no container wrapper:

```bash
git clone https://github.com/alexmchughdev/lookout && cd lookout && ./install.sh
```

Handles Chromium, Ollama, the default vision model (`gemma3:12b`), and the Go
build. Detects apt / pacman / dnf / zypper / brew automatically. Flags:
`--yes` (skip prompts), `--no-model`, `--model NAME`, `--prefix DIR`.

Requires Go 1.22+. Debian 12 and Ubuntu 22.04 ship older Go — the installer
will tell you to grab a current version from https://go.dev/dl/ or via asdf.

### Option 3 — manual

```bash
git clone https://github.com/alexmchughdev/lookout
cd lookout
make build
sudo mv lookout /usr/local/bin/

# Chromium — pick the one for your distro
sudo apt install chromium           # Ubuntu / Debian
sudo pacman -S chromium             # Arch / EndeavourOS / Manjaro
sudo dnf install chromium           # Fedora
sudo zypper install chromium        # openSUSE
brew install --cask chromium        # macOS

ollama pull gemma3:12b
```

## Quick start

```bash
lookout init --url https://myapp.com --email me@example.com
export LOOKOUT_PASSWORD='mypassword'
lookout validate        # sanity-check the spec
lookout run
```

## Using a hosted vision model (Anthropic / OpenAI)

No GPU? Don't want to run Ollama? Use a cloud API instead. You pay per token
(usually fractions of a cent per test), but everything else — the deterministic
browser driving, the spec format, the report — stays identical.

### Anthropic (Claude)

Get a key at https://console.anthropic.com.

```bash
export LOOKOUT_API_KEY=sk-ant-...
lookout run tests.yaml --provider anthropic --model claude-sonnet-4-6
```

Or set it in the spec and skip the flags:

```yaml
model:
  provider: anthropic
  name: claude-sonnet-4-6
  api_key: ""   # leave empty and set LOOKOUT_API_KEY, or paste here
```

### OpenAI

Get a key at https://platform.openai.com/api-keys.

```bash
export LOOKOUT_API_KEY=sk-...
lookout run tests.yaml --provider openai --model gpt-5.4
```

Or in the spec:

```yaml
model:
  provider: openai
  name: gpt-5.4
  api_key: ""
```

### Notes

- **Never commit the key.** Use `LOOKOUT_API_KEY` and keep it out of version
  control. The built-in `.gitignore` catches `.env`.
- **Preflight skips the /api/tags check for cloud providers** — it just
  confirms the key is non-empty. First real test call will surface auth errors.
- **Screenshots are sent to the provider** — full-page PNGs of your app. If
  the app shows PII or other sensitive content, either keep the model local,
  use section filters, or read the provider's data-retention policy.
- **Per-test cost**: a single Claude Sonnet 4.6 judgement on a 1440×900
  screenshot is typically under 2¢. 30 tests costs less than a coffee.

## Apps behind MFA / SSO

If your app uses Microsoft 2FA, Okta, Google SSO, or any flow that needs a
human at login time, automatic login won't work. Use **session auth** instead:
log in once via a headed browser, and lookout reuses the cookies on every run.

```yaml
# in your spec
auth:
  type: session
  session_file: .lookout/session.json   # default; add to .gitignore
```

```bash
lookout auth              # opens headed browser — sign in (including MFA), press Enter
lookout run               # reuses the saved session — no login step
# re-run `lookout auth` when the session expires (typically days)
```

The saved file contains auth cookies — keep it out of version control.

## Usage

**Got a test spec PDF?** The built-in `lookout run spec.pdf` uses a local model
and is hit-or-miss on complex specs. For reliable conversion, paste the prompt
at [`docs/prompts/pdf-to-yaml.md`](docs/prompts/pdf-to-yaml.md) into Claude
(or your LLM of choice) along with the PDF — it emits a clean YAML you can
drop into `lookout run`.

```bash
lookout init                                         # scaffold a lookout.yaml
lookout validate tests.yaml                          # sanity-check a spec
lookout auth                                         # capture login session for MFA/SSO apps
lookout run tests.yaml                               # YAML spec
lookout run spec.pdf --url https://myapp.com         # PDF spec (parsed locally)
lookout run tests.yaml --sections auth,dashboard     # specific sections
lookout run tests.yaml --build abc1234               # tag report with build
lookout run tests.yaml --headed                      # visible browser
lookout run tests.yaml --retry 2                     # retry flaky tests up to 2x
lookout run tests.yaml --junit results.xml           # JUnit XML for CI
lookout run tests.yaml --json   results.json         # machine-readable JSON
lookout run tests.yaml --provider anthropic --api-key sk-ant-...  # Claude API
lookout models                                       # list recommended models

# CI / unattended opt-outs
lookout run tests.yaml --no-open          # don't auto-open the HTML report
lookout run tests.yaml --no-gpu-monitor   # don't pop a GPU-stats window
lookout run tests.yaml --no-screenshots   # minimal report, no embedded images
lookout run tests.yaml --no-preflight     # skip the vision-model reachability check
lookout run tests.yaml --no-report        # skip HTML report generation
```

On an interactive desktop `lookout run` auto-opens the HTML report in your
browser and pops a second terminal running `nvtop` (or equivalent) so you can
watch the vision model light up your GPU. Both auto-detect and silently skip
in CI or headless contexts — no flag needed.

## CI integration

`lookout run` exits non-zero if any test fails, so it slots straight into CI:

```yaml
# .github/workflows/qa.yml
- run: lookout run tests.yaml --junit junit.xml --retry 1 --build ${{ github.sha }}
- uses: actions/upload-artifact@v4
  if: always()
  with:
    name: lookout-report
    path: |
      reports/*.html
      junit.xml
```

## YAML spec format

```yaml
app:
  url: https://myapp.com
  auth:
    type: email_password
    login_path: /login            # override if login page isn't at /login
    email: qa@myapp.com
    password: ""                  # or: export LOOKOUT_PASSWORD
    # continue_button: 'button:has-text("Continue")'

model:
  provider: ollama
  name: gemma3:12b

tests:
  - id: smoke-01
    section: smoke
    url: /
    question: Does the app load without a blank white screen?

  - id: login-01
    section: auth
    url: /login
    question: Is a login form visible with email and password fields?

  - id: dashboard-01
    section: dashboard
    url: /dashboard
    question: Has the dashboard loaded with widgets visible?
    wait_for: '.dashboard-loaded'   # CSS selector to wait for
    wait_ms: 1000                   # extra settle time
    full_page: true                 # default true; set false for viewport-only

  - id: notes-persist
    section: notes
    url: /notes
    question: Does the edited content persist after a page reload?
    pre_action:
      type: type_and_verify
      click_selector: 'text=My Note'
      editor_selector: '[contenteditable="true"]'
      text: LOOKOUT-TEST
```

## Test fields

| Field | Description |
|-------|-------------|
| `id` | Unique identifier |
| `section` | Grouping tag — filter with `--sections` |
| `url` | Path relative to `app.url` |
| `question` | Pass/Fail question for the vision model |
| `wait_for` | CSS selector to wait for before screenshot (SPA hydration) |
| `wait_ms` | Extra delay in ms after navigation / pre-action |
| `full_page` | Capture entire scrollable page (default `true`) |
| `pre_action` | Optional interaction before screenshot (see below) |

## Pre-actions

| Type | Description | Parameters |
|------|-------------|------------|
| `click` | Click an element | `selector`, `wait_ms` |
| `type_and_verify` | Type, save, reload, verify | `click_selector`, `editor_selector`, `text` |
| `open_first` | Click first item in list | `selector`, `fallback_button` |
| `drag` | Drag element (React DnD) | `source`, `target`, `hold_ms`, `reload_after` |
| `new_item` | Click New/Create button | `selector` |
| `select_option` | Click first option | `selector` |
| `reload` | Reload the page | `wait_ms` |
| `wait` | Wait a fixed duration | `ms` |

## Model providers

| Provider | Setup | Cost |
|----------|-------|------|
| `ollama` (default) | `ollama pull gemma3:12b` | Free, local |
| `anthropic` | `--provider anthropic --api-key sk-ant-...` | Per token |
| `openai` | `--provider openai --api-key sk-...` | Per token |

## Environment variables

| Variable | Description |
|----------|-------------|
| `LOOKOUT_EMAIL` | Login email (email_password auth) |
| `LOOKOUT_PASSWORD` | Login password (email_password auth) |
| `LOOKOUT_API_KEY` | API key for anthropic/openai |
| `LOOKOUT_BUILD` | Build ID for report |
| `LOOKOUT_OLLAMA_HOST` | Override the default Ollama host (`http://localhost:11434`) — useful in docker-compose where Ollama is a sibling service |
| `LOOKOUT_TERMINAL` | Override the terminal emulator used for the GPU-stats window |

## Architecture

```
lookout run spec.yaml
       │
       ├─ spec loaded + validated (YAML, or PDF via local vision)
       ├─ vision model preflight (fail fast if Ollama/API unreachable)
       ├─ chromedp launches Chrome at 1440×900
       ├─ auth: deterministic email/password OR restore saved session
       │        (lookout auth captures SSO / MFA sessions once)
       │
       └─ for each test:
              ├─ navigate to URL
              ├─ optional pre-action (click, drag, type, reload, ...)
              ├─ wait_for selector / wait_ms for SPA hydration
              ├─ full-page screenshot captured
              └─ vision model: Pass / Fail / Blocked / Skipped + one-sentence note
                       │
                       ├─ retry on Fail/Blocked (--retry N)
                       └─ HTML + JUnit XML + JSON report outputs
```

## Cross-compile

```bash
make cross
# dist/lookout-linux-amd64
# dist/lookout-darwin-amd64
# dist/lookout-darwin-arm64
# dist/lookout-windows-amd64.exe
```

## Licence

MIT
