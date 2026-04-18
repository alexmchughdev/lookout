#!/usr/bin/env bash
# Run the install script inside a matrix of Linux distros via Docker.
# Each container:
#   - mounts the local repo read-only at /src
#   - copies it to /build (install.sh needs to write there)
#   - runs ./install.sh --yes --no-model --prefix /tmp/lkbin
#   - then verifies ./lookout --version
#
# Requires docker (sudo docker, on systems where your user isn't in the docker group).
# Run: sudo scripts/test-install-matrix.sh
#
# What this checks:
#   - OS/package-manager detection fires correctly
#   - Go toolchain installs and check_go_version passes
#   - Chromium installs
#   - go build succeeds
#   - The resulting binary runs and prints its version
#
# What this does NOT check:
#   - Ollama install (skipped via --no-model; Ollama needs systemd in some paths)
#   - A full `lookout run` end-to-end (needs a display + real model)
set -euo pipefail

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

MATRIX=(
  "ubuntu:24.04"
  "ubuntu:22.04"        # expected to fail check_go_version — proves the guard works
  "debian:13"
  "debian:12"           # expected to fail check_go_version
  "fedora:40"
  "archlinux:latest"
  "opensuse/tumbleweed:latest"
)

pass=0
fail=0
skipped=()

for image in "${MATRIX[@]}"; do
  echo ""
  echo "══════════════════════════════════════════════════════════════════════"
  echo "  $image"
  echo "══════════════════════════════════════════════════════════════════════"

  # Per-distro pre-flight for package managers that need `curl` or `sudo` to
  # even be present before install.sh runs.
  bootstrap=""
  case "$image" in
    ubuntu:*|debian:*)   bootstrap="apt-get update -qq && apt-get install -y -qq curl sudo ca-certificates" ;;
    fedora:*)            bootstrap="dnf install -y -q curl sudo" ;;
    archlinux:*)         bootstrap="pacman -Sy --noconfirm curl sudo" ;;
    opensuse/*)          bootstrap="zypper --non-interactive install curl sudo" ;;
  esac

  if docker run --rm -v "$REPO_ROOT:/src:ro" "$image" bash -c "
    set -e
    $bootstrap >/dev/null 2>&1
    cp -r /src /build
    cd /build
    ./install.sh --yes --no-model --prefix /tmp/lkbin
    /tmp/lkbin/lookout --version
  "; then
    echo "  ✓ $image: install + version"
    pass=$((pass+1))
  else
    echo "  ✗ $image: failed"
    fail=$((fail+1))
  fi
done

echo ""
echo "══════════════════════════════════════════════════════════════════════"
echo "  Result: $pass passed, $fail failed"
echo "══════════════════════════════════════════════════════════════════════"
echo ""
echo "Note: ubuntu:22.04 and debian:12 are EXPECTED to fail the Go version"
echo "check. That failure is the feature — it prevents a worse 'invalid go"
echo "version' error during the build step."

exit $fail
