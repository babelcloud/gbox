#!/bin/bash
# Unified noVNC launcher (host desktop :0 preferred; fallback to virtual :1+XFCE)
# Default: no password (-nopw). Set VNC_PASSWORD to enable password auth.
# Comments are in English.

set -euo pipefail

# -----------------------------
# Config (override via env)
# -----------------------------
USE_HOST_DESKTOP="${USE_HOST_DESKTOP:-1}"     # 1 => prefer host desktop :0, 0 => force virtual :1
DISPLAY_VIRTUAL="${DISPLAY_VIRTUAL:-:1}"      # Virtual display when USE_HOST_DESKTOP=0
RESOLUTION="${RESOLUTION:-2560x1440x24}"      # WxHxD for Xvfb when virtual
VNC_PORT="${VNC_PORT:-5900}"
NOVNC_PORT="${NOVNC_PORT:-6080}"
NOVNC_WEB_DIR="${NOVNC_WEB_DIR:-/usr/share/novnc}"
AUTOLINK_INDEX="${AUTOLINK_INDEX:-1}"         # 1 => symlink index.html -> vnc.html
VNC_PASSWORD="${VNC_PASSWORD:-}"              # empty => no password

# -----------------------------
# Helpers
# -----------------------------
port_listen() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltnp 2>/dev/null | grep -q ":${port} "
  else
    netstat -ltnp 2>/dev/null | grep -q ":${port} "
  fi
}
wait_for_sock() {
  local disp="$1"
  local sock="/tmp/.X11-unix/X${disp#:}"
  local i=0
  while [ $i -lt 120 ]; do
    [ -S "$sock" ] && return 0
    sleep 0.1
    i=$((i+1))
  done
  return 1
}
ensure_pkg() {
  #sudo apt-get update -y
  sudo apt-get install -y "$@"
}
start_websockify() {
  local port="$1" target="$2"
  if port_listen "$port"; then
    pkill -f "websockify .* ${port} " 2>/dev/null || true
    sleep 0.5
  fi
  nohup websockify --libserver --web "${NOVNC_WEB_DIR}" "${port}" "${target}" >/dev/null 2>&1 &
  local i=0
  while [ $i -lt 80 ]; do
    port_listen "${port}" && return 0
    sleep 0.1
    i=$((i+1))
  done
  echo "noVNC failed to bind ${port}" >&2
  return 1
}
x11vnc_args_for_display() {
  local disp="$1"
  local args=(-display "$disp" -rfbport "$VNC_PORT" -forever -shared -xrandr -ncache 0 -noxdamage)
  if [ -n "$VNC_PASSWORD" ]; then
    mkdir -p ~/.vnc
    x11vnc -storepasswd "$VNC_PASSWORD" ~/.vnc/passwd >/dev/null 2>&1
    args+=(-rfbauth ~/.vnc/passwd)
  else
    args+=(-nopw)
  fi
  printf '%s\n' "${args[@]}"
}

# -----------------------------
# 1) Install dependencies
# -----------------------------
ensure_pkg x11vnc novnc python3-websockify
# Virtual desktop deps (only used when falling back)
ensure_pkg xvfb xfce4 xfce4-terminal xfce4-goodies dbus-x11 x11-apps

[ -d "$NOVNC_WEB_DIR" ] || { echo "noVNC dir not found: $NOVNC_WEB_DIR"; exit 1; }

# -----------------------------
# 2) Decide mode (prefer host :0 if socket exists)
# -----------------------------
HOST_XSOCK="/tmp/.X11-unix/X0"
if [ "$USE_HOST_DESKTOP" = "1" ] && [ -S "$HOST_XSOCK" ]; then
  MODE="host"; TARGET_DISPLAY=":0"
else
  MODE="virtual"; TARGET_DISPLAY="$DISPLAY_VIRTUAL"
fi
echo "[mode] ${MODE} (DISPLAY ${TARGET_DISPLAY})"

# -----------------------------
# 3A) Host desktop (:0)
# -----------------------------
if [ "$MODE" = "host" ]; then
  # Clean stale processes
  pkill -x x11vnc 2>/dev/null || true
  pkill -f "websockify .* ${NOVNC_PORT} " 2>/dev/null || true

  # Build x11vnc args and include -auth guess for :0
  # shellcheck disable=SC2207
  VNC_ARGS=($(x11vnc_args_for_display "$TARGET_DISPLAY"))
  VNC_ARGS+=(-auth guess)

  nohup x11vnc "${VNC_ARGS[@]}" >/dev/null 2>&1 &
  i=0
  while [ $i -lt 80 ]; do
    port_listen "$VNC_PORT" && break
    sleep 0.1
    i=$((i+1))
  done
  port_listen "$VNC_PORT" || { echo "x11vnc not listening on ${VNC_PORT}"; exit 1; }

  start_websockify "$NOVNC_PORT" "localhost:${VNC_PORT}"

  if [ "$AUTOLINK_INDEX" = "1" ] && [ -f "${NOVNC_WEB_DIR}/vnc.html" ]; then
    ln -sf "${NOVNC_WEB_DIR}/vnc.html" "${NOVNC_WEB_DIR}/index.html" 2>/dev/null || true
  fi

  echo ""
  echo "-----------------------------------------"
  echo "Ready (host :0)."
  echo "VNC:     localhost:${VNC_PORT}"
  echo "noVNC:   http://localhost:${NOVNC_PORT}/vnc.html?autoconnect=1&host=localhost&port=${NOVNC_PORT}"
  if [ -z "$VNC_PASSWORD" ]; then
    echo "Auth:    none (-nopw) [WARNING: insecure on untrusted networks]"
  else
    echo "Auth:    password (~/.vnc/passwd)"
  fi
  echo "-----------------------------------------"
  exit 0
fi

# -----------------------------
# 3B) Virtual desktop (:1 + XFCE)
# -----------------------------
# Start Xvfb
if [ -S "/tmp/.X11-unix/X${TARGET_DISPLAY#:}" ]; then
  echo "· ${TARGET_DISPLAY} exists, skip Xvfb"
else
  nohup Xvfb "${TARGET_DISPLAY}" -screen 0 "${RESOLUTION}" -ac +extension RANDR -nolisten tcp -noreset >/dev/null 2>&1 &
  wait_for_sock "${TARGET_DISPLAY}" || { echo "Xvfb did not come up"; exit 1; }
fi
echo "✓ X ready: ${TARGET_DISPLAY}"

# Ensure XDG_RUNTIME_DIR and start XFCE once
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/tmp/xdg-$(id -u)}"
mkdir -p "$XDG_RUNTIME_DIR" && chmod 700 "$XDG_RUNTIME_DIR" || true
if ! pgrep -a xfwm4 >/dev/null 2>&1 && ! pgrep -a startxfce4 >/dev/null 2>&1; then
  DISPLAY="${TARGET_DISPLAY}" nohup dbus-launch startxfce4 >/tmp/xfce.log 2>&1 &
  sleep 2
fi

# Start x11vnc on virtual display
pkill -x x11vnc 2>/dev/null || true
sleep 0.3
# shellcheck disable=SC2207
VNC_ARGS=($(x11vnc_args_for_display "$TARGET_DISPLAY"))
nohup x11vnc "${VNC_ARGS[@]}" >/dev/null 2>&1 &
i=0
while [ $i -lt 80 ]; do
  port_listen "$VNC_PORT" && break
  sleep 0.1
  i=$((i+1))
done
port_listen "$VNC_PORT" || { echo "x11vnc not listening on ${VNC_PORT}"; exit 1; }

# Start websockify
start_websockify "$NOVNC_PORT" "localhost:${VNC_PORT}"

# Optional: default to UI
if [ "$AUTOLINK_INDEX" = "1" ] && [ -f "${NOVNC_WEB_DIR}/vnc.html" ]; then
  ln -sf "${NOVNC_WEB_DIR}/vnc.html" "${NOVNC_WEB_DIR}/index.html" 2>/dev/null || true
fi

echo ""
echo "-----------------------------------------"
echo "Ready (virtual XFCE)."
echo "DISPLAY: ${TARGET_DISPLAY} (${RESOLUTION%x*})"
echo "VNC:     localhost:${VNC_PORT}"
echo "noVNC:   http://localhost:${NOVNC_PORT}/vnc.html?autoconnect=1&host=localhost&port=${NOVNC_PORT}"
echo "Logs:    /tmp/xfce.log"
if [ -z "$VNC_PASSWORD" ]; then
  echo "Auth:    none (-nopw) [WARNING: insecure on untrusted networks]"
else
  echo "Auth:    password (~/.vnc/passwd)"
fi
echo "-----------------------------------------"


