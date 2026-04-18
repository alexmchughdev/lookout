#!/usr/bin/env bash
# lookout installer — https://github.com/alexmchughdev/lookout
#
# Usage:
#   ./install.sh                    # interactive
#   ./install.sh --yes              # auto-approve prompts
#   ./install.sh --no-model         # skip `ollama pull gemma3:12b`
#   ./install.sh --prefix ~/bin     # install somewhere other than /usr/local/bin
#   ./install.sh --model qwen2.5vl:7b
set -euo pipefail

# ── colours ───────────────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'
  GREEN=$'\033[32m'; YELLOW=$'\033[33m'; CYAN=$'\033[36m'; RESET=$'\033[0m'
else
  BOLD=""; DIM=""; RED=""; GREEN=""; YELLOW=""; CYAN=""; RESET=""
fi

info()  { printf "${CYAN}▸${RESET} %s\n" "$*"; }
ok()    { printf "${GREEN}✓${RESET} %s\n" "$*"; }
warn()  { printf "${YELLOW}⚠${RESET} %s\n" "$*"; }
err()   { printf "${RED}✗${RESET} %s\n" "$*" >&2; }
step()  { printf "\n${BOLD}%s${RESET}\n" "$*"; }

# ── flags ─────────────────────────────────────────────────────────────────────
AUTO_YES=0
PULL_MODEL=1
MODEL="gemma3:12b"
PREFIX="/usr/local/bin"

while [[ $# -gt 0 ]]; do
  case "$1" in
    -y|--yes)       AUTO_YES=1 ;;
    --no-model)     PULL_MODEL=0 ;;
    --model)        MODEL="$2"; shift ;;
    --prefix)       PREFIX="$2"; shift ;;
    -h|--help)
      sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) err "unknown flag: $1"; exit 2 ;;
  esac
  shift
done

confirm() {
  if [[ $AUTO_YES -eq 1 ]]; then return 0; fi
  local prompt="$1 [y/N] "
  read -r -p "$prompt" reply
  [[ "$reply" =~ ^[Yy]$ ]]
}

# ── ASCII banner ──────────────────────────────────────────────────────────────
cat <<'EOF'

    __                __              __
   / /   ____  ____  / /______  __  _/ /_
  / /   / __ \/ __ \/ //_/ __ \/ / / / __/
 / /___/ /_/ / /_/ / ,< / /_/ / /_/ / /_
/_____/\____/\____/_/|_|\____/\__,_/\__/

 visual QA · local-first · single binary

EOF

# ── detect OS ─────────────────────────────────────────────────────────────────
OS="unknown"
case "$(uname -s)" in
  Linux*)  OS="linux" ;;
  Darwin*) OS="macos" ;;
  *)       err "unsupported OS: $(uname -s)"; exit 1 ;;
esac
info "OS: $OS"

# ── package manager ───────────────────────────────────────────────────────────
PKG=""
if   command -v apt-get >/dev/null 2>&1; then PKG="apt"
elif command -v pacman  >/dev/null 2>&1; then PKG="pacman"
elif command -v dnf     >/dev/null 2>&1; then PKG="dnf"
elif command -v brew    >/dev/null 2>&1; then PKG="brew"
fi
if [[ -n "$PKG" ]]; then info "package manager: $PKG"; fi

install_pkg() {
  local name="$1"
  case "$PKG" in
    apt)    sudo apt-get update -qq && sudo apt-get install -y "$name" ;;
    pacman) sudo pacman -S --noconfirm "$name" ;;
    dnf)    sudo dnf install -y "$name" ;;
    brew)   brew install "$name" ;;
    *)      err "no supported package manager; install $name manually"; return 1 ;;
  esac
}

# ── 1. Go toolchain (build dependency) ────────────────────────────────────────
step "1/5  Go toolchain"
if command -v go >/dev/null 2>&1; then
  ok "Go $(go version | awk '{print $3}') already installed"
else
  warn "Go not found — needed to build lookout"
  if confirm "Install Go via $PKG?"; then
    case "$PKG" in
      apt|dnf) install_pkg golang ;;
      pacman)  install_pkg go ;;
      brew)    install_pkg go ;;
      *)       err "install Go from https://go.dev/dl/ and re-run"; exit 1 ;;
    esac
    ok "Go installed"
  else
    err "Go is required to build. Aborting."; exit 1
  fi
fi

# ── 2. Chromium ───────────────────────────────────────────────────────────────
step "2/5  Chromium"
CHROME_FOUND=""
for c in chromium-browser chromium google-chrome google-chrome-stable; do
  if command -v "$c" >/dev/null 2>&1; then CHROME_FOUND="$c"; break; fi
done
# macOS app bundle
if [[ -z "$CHROME_FOUND" && -d "/Applications/Chromium.app" ]]; then
  CHROME_FOUND="/Applications/Chromium.app"
fi
if [[ -z "$CHROME_FOUND" && -d "/Applications/Google Chrome.app" ]]; then
  CHROME_FOUND="/Applications/Google Chrome.app"
fi

if [[ -n "$CHROME_FOUND" ]]; then
  ok "Chromium/Chrome found: $CHROME_FOUND"
else
  warn "No Chromium/Chrome binary found"
  if confirm "Install Chromium via $PKG?"; then
    case "$PKG" in
      apt)    install_pkg chromium-browser || install_pkg chromium ;;
      pacman) install_pkg chromium ;;
      dnf)    install_pkg chromium ;;
      brew)   brew install --cask chromium ;;
      *)      err "install Chromium manually and re-run"; exit 1 ;;
    esac
    ok "Chromium installed"
  else
    warn "Skipping — lookout will fail until you install Chromium"
  fi
fi

# ── 3. Ollama ─────────────────────────────────────────────────────────────────
step "3/5  Ollama (local vision model host)"
if command -v ollama >/dev/null 2>&1; then
  ok "Ollama found: $(ollama --version 2>/dev/null | head -1)"
else
  warn "Ollama not found"
  if confirm "Install Ollama (https://ollama.com)?"; then
    curl -fsSL https://ollama.com/install.sh | sh
    ok "Ollama installed"
  else
    warn "Skipping — you'll need Ollama OR a hosted API (anthropic/openai) to run tests"
  fi
fi

# ── 4. Vision model ───────────────────────────────────────────────────────────
step "4/5  Vision model ($MODEL)"
if [[ $PULL_MODEL -eq 1 ]] && command -v ollama >/dev/null 2>&1; then
  if ollama list 2>/dev/null | awk '{print $1}' | grep -qx "$MODEL"; then
    ok "$MODEL already pulled"
  else
    # Check Ollama daemon is up
    if ! curl -fsS http://localhost:11434/api/tags >/dev/null 2>&1; then
      warn "Ollama daemon not running — start it with: ollama serve"
      warn "Skipping model pull. Run: ollama pull $MODEL"
    else
      if confirm "Pull $MODEL (~8GB)?"; then
        ollama pull "$MODEL"
        ok "$MODEL ready"
      else
        warn "Skipping model pull. Run later: ollama pull $MODEL"
      fi
    fi
  fi
else
  info "Skipping model pull"
fi

# ── 5. Build & install lookout ────────────────────────────────────────────────
step "5/5  Build & install lookout → $PREFIX"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

if [[ ! -f go.mod ]]; then
  err "This script must be run from the lookout source tree (go.mod missing)."
  err "Clone the repo first:  git clone https://github.com/alexmchughdev/lookout"
  exit 1
fi

info "go mod tidy"
go mod tidy

info "go build"
go build -ldflags "-s -w" -o lookout .
ok "built ./lookout ($(du -h lookout | awk '{print $1}'))"

if [[ ! -d "$PREFIX" ]]; then
  mkdir -p "$PREFIX" 2>/dev/null || true
fi

NEEDS_SUDO=""
if [[ ! -w "$PREFIX" ]]; then NEEDS_SUDO="sudo"; fi

if confirm "Install to $PREFIX/lookout?${NEEDS_SUDO:+ (sudo required)}"; then
  $NEEDS_SUDO install -m 0755 lookout "$PREFIX/lookout"
  ok "installed: $PREFIX/lookout"
else
  info "Binary left at $SCRIPT_DIR/lookout"
fi

# ── Done ──────────────────────────────────────────────────────────────────────
cat <<EOF

${GREEN}${BOLD}Installed.${RESET} Next steps:

  ${DIM}# scaffold a spec${RESET}
  lookout init --url https://myapp.com --email me@company.com

  ${DIM}# apps behind MFA / SSO — set auth.type: session in the YAML, then:${RESET}
  lookout auth

  ${DIM}# run${RESET}
  lookout run

Docs: https://github.com/alexmchughdev/lookout
EOF
