#!/bin/bash
# Initialize xdotool environment on Linux desktop
# - Install xdotool and wmctrl
# - Detect DISPLAY (:0 preferred, fallback :1)
# - Export minimal env vars for X client tools

set -euo pipefail

detect_display() {
  if [ -S /tmp/.X11-unix/X0 ]; then
    echo ":0"
  elif [ -S /tmp/.X11-unix/X1 ]; then
    echo ":1"
  else
    # Fallback to :1 (assumed created by other init script)
    echo ":1"
  fi
}

ensure_pkg() {
  if command -v apt-get >/dev/null 2>&1; then
    sudo apt-get update -y || true
    sudo apt-get install -y "$@"
    return 0
  fi
  if command -v yum >/dev/null 2>&1; then
    sudo yum install -y "$@"
    return 0
  fi
  if command -v dnf >/dev/null 2>&1; then
    sudo dnf install -y "$@"
    return 0
  fi
  if command -v pacman >/dev/null 2>&1; then
    sudo pacman -Sy --noconfirm "$@"
    return 0
  fi
  echo "Unsupported package manager. Please install: $*" >&2
  exit 1
}

ensure_pkg xdotool wmctrl

export DISPLAY="${DISPLAY:-$(detect_display)}"

# Try to set XAUTHORITY if available
USER_XAUTH="$HOME/.Xauthority"
ROOT_XAUTH="/root/.Xauthority"
if [ -f "$USER_XAUTH" ]; then
  export XAUTHORITY="$USER_XAUTH"
elif [ -f "$ROOT_XAUTH" ]; then
  export XAUTHORITY="$ROOT_XAUTH"
fi

# Smoke test (best-effort)
if ! xdotool version >/dev/null 2>&1; then
  echo "xdotool not available in PATH after installation" >&2
  exit 1
fi

echo "xdotool initialized. DISPLAY=$DISPLAY"


