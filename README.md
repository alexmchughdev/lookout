# lookout

```
    __                __              __
   / /   ____  ____  / /______  __  _/ /_
  / /   / __ \/ __ \/ //_/ __ \/ / / / __/
 / /___/ /_/ / /_/ / ,< / /_/ / /_/ / /_
/_____/\____/\____/_/|_|\____/\__,_/\__/

 visual QA · local-first · single binary
```

**lookout** automates E2E visual QA. chromedp navigates your app deterministically. A local vision model looks at each screenshot and returns a Pass/Fail verdict. No agent loops. No cloud required. Single binary.

## Install

```bash
git clone https://github.com/AlexMcHugh1/lookout
cd lookout
make build
sudo mv lookout /usr/local/bin/
```

Requires Chromium and Ollama with a vision model:

```bash
sudo apt install chromium-browser   # or: brew install --cask chromium
ollama pull gemma3:12b
```

## Quick start

```bash
lookout init --url https://myapp.com --email me@example.com
export LOOKOUT_PASSWORD='mypassword'
lookout run
```

## Usage

```bash
lookout run tests.yaml                              # YAML spec
lookout run spec.pdf --url https://myapp.com         # PDF spec (parsed locally)
lookout run tests.yaml --sections auth,dashboard     # specific sections
lookout run tests.yaml --build abc1234               # tag report with build
lookout run tests.yaml --debug                       # embed all screenshots
lookout run tests.yaml --headed                      # visible browser
lookout run tests.yaml --provider anthropic --api-key sk-ant-...  # Claude API
lookout models                                       # list recommended models
```

## YAML spec format

```yaml
app:
  url: https://myapp.com
  auth:
    type: email_password
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
| `LOOKOUT_EMAIL` | Login email |
| `LOOKOUT_PASSWORD` | Login password |
| `LOOKOUT_API_KEY` | API key for anthropic/openai |
| `LOOKOUT_BUILD` | Build ID for report |

## Architecture

```
lookout run spec.yaml
       │
       ├─ spec loaded (YAML or PDF parsed via Ollama vision)
       ├─ chromedp launches Chrome (zero runtime deps)
       ├─ deterministic login (two-step auth aware)
       │
       └─ for each test:
              ├─ navigate to URL
              ├─ optional pre-action (click, drag, type)
              ├─ screenshot captured
              └─ vision model: Pass / Fail + one-sentence note
                       │
                       └─ HTML report with embedded screenshots
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
