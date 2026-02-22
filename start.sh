#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKIP_INSTALL=false

usage() {
  cat <<'EOF'
Usage: ./start.sh [--skip-install]

Environment:
  LPG_PROVIDER                Upstream provider: stub (default), vllm_local, mimo_online, openai_compatible (alias: llamacpp_local)
  LPG_PROVIDER_TIMEOUT        Optional provider timeout duration (for example 2s, 1500ms)
  LPG_ALLOW_RAW_FORWARDING    Optional bool for low-risk raw forwarding (default: false)
  LPG_CRITICAL_LOCAL_ONLY     Optional bool to force local-only critical handling (default: false)
  LPG_VLLM_BASE_URL           Required when LPG_PROVIDER=vllm_local (GPU-backed optional path)
  LPG_VLLM_MODEL              Optional fallback model when LPG_PROVIDER=vllm_local
  LPG_MIMO_BASE_URL           Required when LPG_PROVIDER=mimo_online
  LPG_MIMO_API_KEY            Required when LPG_PROVIDER=mimo_online
  LPG_MIMO_MODEL              Optional fallback model when LPG_PROVIDER=mimo_online (default: mimo-v2-flash)
  LPG_UPSTREAM_BASE_URL       Required when LPG_PROVIDER=openai_compatible
  LPG_UPSTREAM_API_KEY        Optional API key for LPG_PROVIDER=openai_compatible
  LPG_UPSTREAM_MODEL          Optional fallback model for LPG_PROVIDER=openai_compatible
  LPG_UPSTREAM_API_KEY_HEADER Optional auth header name (default: Authorization)
  LPG_UPSTREAM_API_KEY_PREFIX Optional auth prefix (default: Bearer)
  LPG_UPSTREAM_CHAT_PATH      Optional chat path (default: /v1/chat/completions)
  LPG_LOCAL_ABSTRACTION_BASE_URL       Optional local abstraction OpenAI-compatible base URL
  LPG_LOCAL_ABSTRACTION_API_KEY        Optional API key for local abstraction endpoint
  LPG_LOCAL_ABSTRACTION_MODEL          Required if local abstraction base URL is set
  LPG_LOCAL_ABSTRACTION_API_KEY_HEADER Optional auth header name for local abstraction (default: Authorization)
  LPG_LOCAL_ABSTRACTION_API_KEY_PREFIX Optional auth prefix for local abstraction (default: Bearer)
  LPG_LOCAL_ABSTRACTION_CHAT_PATH      Optional chat path for local abstraction (default: /v1/chat/completions)

Options:
  --skip-install  Skip dependency/tool installation and go mod tidy
  -h, --help      Show this help message
EOF
}

for arg in "$@"; do
  case "$arg" in
    --skip-install)
      SKIP_INSTALL=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $arg"
      usage
      exit 1
      ;;
  esac
done

if ! command -v go >/dev/null 2>&1; then
  echo "Error: Go is required (go1.24.13+)."
  exit 1
fi

# Ensure Go-installed binaries are discoverable.
GO_BIN_DIR="$(go env GOBIN)"
if [[ -z "$GO_BIN_DIR" ]]; then
  GO_BIN_DIR="$(go env GOPATH)/bin"
fi
export PATH="$PATH:$GO_BIN_DIR"

install_if_missing() {
  local bin_name="$1"
  local module_path="$2"

  if command -v "$bin_name" >/dev/null 2>&1; then
    echo "$bin_name already installed"
    return
  fi

  echo "Installing $bin_name..."
  go install "$module_path"
}

if [[ "$SKIP_INSTALL" == "false" ]]; then
  install_if_missing golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  install_if_missing govulncheck golang.org/x/vuln/cmd/govulncheck@latest

  if command -v gitleaks >/dev/null 2>&1; then
    echo "gitleaks already installed"
  else
    echo "Installing gitleaks..."
    if ! go install github.com/gitleaks/gitleaks/v8@latest; then
      go install github.com/zricethezav/gitleaks/v8@latest
    fi
  fi

  # Re-export PATH in case tools were installed into GO_BIN_DIR during this run.
  export PATH="$PATH:$GO_BIN_DIR"

  echo "Syncing module dependencies..."
  go mod tidy
else
  echo "Skipping dependency installation and module sync (--skip-install)"
fi

echo "Starting LPG..."
exec go run "$ROOT_DIR/cmd/lpg"
