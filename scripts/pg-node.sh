#!/usr/bin/env bash
#
# pg-node.sh — installer/manager for the PasarGuard multi-tenant node agent (fork).
#
# This is NOT compatible with the upstream PasarGuard node; it manages the
# shared-core, multi-tenant agent (HTTP control plane + optional PasarGuard-compat
# gRPC for core config). It is self-contained (no external shared libs).
#
# One-line install (after hosting this repo):
#   sudo bash -c "$(curl -sL https://raw.githubusercontent.com/loopy-iri/NodeAgent/main/scripts/pg-node.sh)" @ install
#
set -euo pipefail

# ---------- globals ----------
APP_NAME="${APP_NAME:-pg-node}"
INSTALL_DIR="/opt"
APP_DIR="${APP_DIR:-$INSTALL_DIR/$APP_NAME}"
DATA_DIR="${DATA_DIR:-/var/lib/$APP_NAME}"
ENV_FILE="$APP_DIR/.env"
COMPOSE_FILE="$APP_DIR/docker-compose.agent.yml"
SSL_CERT_FILE="$DATA_DIR/certs/ssl_cert.pem"
SSL_KEY_FILE="$DATA_DIR/certs/ssl_key.pem"
FIXED_CONFIG_FILE="$DATA_DIR/fixed-config.json"
REPO_URL="${REPO_URL:-https://github.com/loopy-iri/NodeAgent.git}"   # this fork's repo
COMPOSE=""

# ---------- helpers ----------
colorized_echo() {
    local color="$1" text="$2" code
    case "$color" in
    red) code=31 ;; green) code=32 ;; yellow) code=33 ;;
    blue) code=34 ;; magenta) code=35 ;; cyan) code=36 ;; *) code=0 ;;
    esac
    printf "\033[1;%sm%s\033[0m\n" "$code" "$text"
}
die() { colorized_echo red "$1"; exit 1; }

check_root() { [ "$(id -u)" -eq 0 ] || die "This command must run as root (use sudo)."; }

detect_os() {
    if [ -r /etc/os-release ]; then . /etc/os-release; OS_ID="${ID:-unknown}"; else OS_ID="unknown"; fi
    if command -v apt-get >/dev/null 2>&1; then PKG="apt";
    elif command -v dnf >/dev/null 2>&1; then PKG="dnf";
    elif command -v yum >/dev/null 2>&1; then PKG="yum";
    elif command -v pacman >/dev/null 2>&1; then PKG="pacman";
    elif command -v zypper >/dev/null 2>&1; then PKG="zypper";
    elif command -v apk >/dev/null 2>&1; then PKG="apk";
    else PKG=""; fi
}

install_package() {
    local pkg="$1"
    case "$PKG" in
    apt) apt-get update -qq && apt-get install -y "$pkg" ;;
    dnf) dnf install -y "$pkg" ;;
    yum) yum install -y "$pkg" ;;
    pacman) pacman -Sy --noconfirm "$pkg" ;;
    zypper) zypper install -y "$pkg" ;;
    apk) apk add --no-cache "$pkg" ;;
    *) die "Unknown package manager; install '$pkg' manually." ;;
    esac
}

need() {
    local bin="$1" pkg="${2:-$1}"
    command -v "$bin" >/dev/null 2>&1 || { colorized_echo yellow "Installing $pkg..."; install_package "$pkg"; }
}

install_docker() {
    if command -v docker >/dev/null 2>&1; then return; fi
    colorized_echo blue "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker 2>/dev/null || true
}

detect_compose() {
    if docker compose version >/dev/null 2>&1; then COMPOSE="docker compose";
    elif command -v docker-compose >/dev/null 2>&1; then COMPOSE="docker-compose";
    else die "docker compose not found."; fi
}

gen_uuid() {
    cat /proc/sys/kernel/random/uuid 2>/dev/null || uuidgen 2>/dev/null || \
        python3 -c "import uuid;print(uuid.uuid4())" 2>/dev/null || die "cannot generate uuid"
}
gen_key() { openssl rand -hex 32 2>/dev/null || gen_uuid; }

public_ip() {
    curl -s -4 --fail --max-time 5 ifconfig.io 2>/dev/null || \
        curl -s -6 --fail --max-time 5 ifconfig.io 2>/dev/null || echo "127.0.0.1"
}

compose_cmd() { ( cd "$APP_DIR" && $COMPOSE --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@" ); }

is_installed() { [ -d "$APP_DIR" ] && [ -f "$COMPOSE_FILE" ]; }
require_installed() { is_installed || die "node is not installed. Run: $APP_NAME install"; }

# ---------- cert ----------
gen_self_signed_cert() {
    need openssl openssl
    local ip; ip="$(public_ip)"
    local san="DNS:localhost,IP:127.0.0.1"
    [ "$ip" != "127.0.0.1" ] && san="$san,IP:$ip"
    [ -n "${INSTALL_SAN_ENTRIES:-}" ] && san="$san,$INSTALL_SAN_ENTRIES"
    mkdir -p "$DATA_DIR/certs"; chmod 700 "$DATA_DIR/certs" 2>/dev/null || true
    colorized_echo blue "Generating self-signed certificate (SAN: $san)..."
    openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 \
        -keyout "$SSL_KEY_FILE" -out "$SSL_CERT_FILE" -days 3650 -nodes \
        -subj "/CN=$ip" -addext "subjectAltName = $san" >/dev/null 2>&1 \
        || die "openssl certificate generation failed"
    chmod 600 "$SSL_KEY_FILE" 2>/dev/null || true
    colorized_echo green "✓ Certificate: $SSL_CERT_FILE"
}

# ---------- sources ----------
fetch_sources() {
    if [ -f "Dockerfile.agent" ] && [ -f "docker-compose.agent.yml" ]; then
        SRC_DIR="$(pwd)"
    else
        need git git
        colorized_echo blue "Cloning $REPO_URL ..."
        rm -rf "$APP_DIR/src"; git clone --depth 1 "$REPO_URL" "$APP_DIR/src"
        SRC_DIR="$APP_DIR/src"
    fi
}

write_fixed_config() {
    [ -f "$FIXED_CONFIG_FILE" ] && { colorized_echo cyan "Keeping existing $FIXED_CONFIG_FILE"; return; }
    if [ -f "$SRC_DIR/configs/fixed-config.example.json" ]; then
        cp "$SRC_DIR/configs/fixed-config.example.json" "$FIXED_CONFIG_FILE"
    else
        cat >"$FIXED_CONFIG_FILE" <<'JSON'
{
  "log": { "loglevel": "warning" },
  "inbounds": [
    { "tag": "vless-in", "listen": "0.0.0.0", "port": 443, "protocol": "vless",
      "settings": { "clients": [], "decryption": "none" }, "streamSettings": { "network": "tcp" } }
  ],
  "outbounds": [ { "tag": "direct", "protocol": "freedom" } ]
}
JSON
    fi
    colorized_echo green "✓ Fixed core config: $FIXED_CONFIG_FILE (edit to taste)"
}

write_env() {
    local master="$1" core="$2" http_port="$3" grpc_port="$4"
    umask 077
    cat >"$ENV_FILE" <<EOF
APP_NAME=$APP_NAME
DATA_DIR=$DATA_DIR
PG_AGENT_HTTP_ADDR=:$http_port
PG_AGENT_GRPC_ADDR=:$grpc_port
PG_AGENT_MASTER_KEY=$master
PG_AGENT_CORE_KEY=$core
PG_AGENT_TENANT_DB=/var/lib/pg-node/tenants.bolt
PG_AGENT_FIXED_CONFIG=/var/lib/pg-node/fixed-config.json
PG_AGENT_ENFORCE_INTERVAL=10s
SSL_CERT_FILE=/var/lib/pg-node/certs/ssl_cert.pem
SSL_KEY_FILE=/var/lib/pg-node/certs/ssl_key.pem
XRAY_EXECUTABLE_PATH=/usr/local/bin/xray
XRAY_ASSETS_PATH=/usr/local/share/xray
EOF
    umask 022
    colorized_echo green "✓ Wrote $ENV_FILE"
}

# ---------- commands ----------
install_command() {
    check_root
    local master="" core="" http_port="8090" grpc_port="62050"
    while [[ $# -gt 0 ]]; do case "$1" in
        --master-key) master="$2"; shift 2 ;;
        --core-key) core="$2"; shift 2 ;;
        --http-port) http_port="$2"; shift 2 ;;
        --grpc-port) grpc_port="$2"; shift 2 ;;
        --san-entries) INSTALL_SAN_ENTRIES="$2"; shift 2 ;;
        --repo) REPO_URL="$2"; shift 2 ;;
        -y|--yes) AUTO_CONFIRM=true; shift ;;
        *) die "Unknown install option: $1" ;;
    esac; done

    detect_os
    need curl curl; need openssl openssl
    install_docker; detect_compose

    [ -z "$master" ] && master="$(gen_key)"
    [ -z "$core" ] && core=""   # core key empty => PasarGuard-compat gRPC disabled

    mkdir -p "$APP_DIR" "$DATA_DIR"
    fetch_sources
    cp "$SRC_DIR/Dockerfile.agent" "$APP_DIR/" 2>/dev/null || true
    cp "$SRC_DIR/docker-compose.agent.yml" "$APP_DIR/"
    # The build context must contain the source; build from SRC_DIR.
    write_fixed_config
    [ -f "$SSL_CERT_FILE" ] || gen_self_signed_cert
    write_env "$master" "$core" "$http_port" "$grpc_port"

    colorized_echo blue "Building and starting the node agent..."
    ( cd "$SRC_DIR" && $COMPOSE --env-file "$ENV_FILE" -f docker-compose.agent.yml up -d --build )

    install_node_script || true

    local ip; ip="$(public_ip)"
    colorized_echo blue "================================"
    colorized_echo green "PasarGuard node agent is up."
    colorized_echo magenta "  Address (HTTPS control):  https://$ip:$http_port"
    [ -n "$core" ] && colorized_echo magenta "  gRPC (PasarGuard-compat): $ip:$grpc_port"
    colorized_echo magenta "  Master key (register in main panel):"
    colorized_echo red "    $master"
    [ -n "$core" ] && { colorized_echo magenta "  Core key (for PasarGuard core management):"; colorized_echo red "    $core"; }
    colorized_echo magenta "  Node certificate (paste into the main panel when adding the node):"
    cat "$SSL_CERT_FILE"
    colorized_echo blue "================================"
}

install_node_script() {
    local target="/usr/local/bin/$APP_NAME"
    if [ -f "$SRC_DIR/scripts/pg-node.sh" ]; then
        install -m 755 "$SRC_DIR/scripts/pg-node.sh" "$target" 2>/dev/null && \
            colorized_echo green "✓ CLI installed: $target ($APP_NAME ...)"
    fi
}
uninstall_node_script() { rm -f "/usr/local/bin/$APP_NAME"; }

up_command()      { require_installed; detect_compose; compose_cmd up -d; colorized_echo green "node started."; }
down_command()    { require_installed; detect_compose; compose_cmd down; colorized_echo green "node stopped."; }
restart_command() { require_installed; detect_compose; compose_cmd down; compose_cmd up -d; colorized_echo green "node restarted."; }
status_command()  { require_installed; detect_compose; compose_cmd ps; }
logs_command()    { require_installed; detect_compose; compose_cmd logs -f --tail=200; }
edit_command()    { require_installed; "${EDITOR:-nano}" "$COMPOSE_FILE"; }
edit_env_command(){ require_installed; "${EDITOR:-nano}" "$ENV_FILE"; }

update_command() {
    check_root; require_installed; detect_compose
    if [ -d "$APP_DIR/src" ]; then ( cd "$APP_DIR/src" && git pull --ff-only || true ); fi
    SRC_DIR="${APP_DIR}/src"; [ -d "$SRC_DIR" ] || SRC_DIR="$APP_DIR"
    ( cd "$SRC_DIR" && $COMPOSE --env-file "$ENV_FILE" -f docker-compose.agent.yml up -d --build )
    colorized_echo green "node updated."
}

renew_cert_command() {
    check_root; require_installed
    gen_self_signed_cert
    colorized_echo yellow "Restart the node and re-pin the new cert in the main panel."
    colorized_echo magenta "New certificate:"; cat "$SSL_CERT_FILE"
    restart_command
}

core_update_command() {
    check_root; require_installed
    local version="${1:-latest}"
    need curl curl; need unzip unzip
    local arch; case "$(uname -m)" in
        x86_64|amd64) arch="64" ;; aarch64|arm64) arch="arm64-v8a" ;;
        armv7l) arch="arm32-v7a" ;; *) die "unsupported arch $(uname -m)" ;;
    esac
    [ "$version" = "latest" ] && version="$(curl -s https://api.github.com/repos/XTLS/Xray-core/releases/latest | grep -oE '"tag_name":[[:space:]]*"[^"]+"' | head -n1 | sed -E 's/.*"([^"]+)"$/\1/')"
    mkdir -p "$DATA_DIR/xray-core"; cd "$DATA_DIR/xray-core"
    colorized_echo blue "Downloading Xray-core $version ($arch)..."
    curl -fsSL "https://github.com/XTLS/Xray-core/releases/download/${version}/Xray-linux-${arch}.zip" -o xray.zip || die "download failed"
    unzip -o xray.zip >/dev/null 2>&1 || die "extract failed"; rm -f xray.zip
    # Point the agent at the host-mounted xray binary.
    sed -i "s|^XRAY_EXECUTABLE_PATH=.*|XRAY_EXECUTABLE_PATH=/var/lib/pg-node/xray-core/xray|" "$ENV_FILE"
    sed -i "s|^XRAY_ASSETS_PATH=.*|XRAY_ASSETS_PATH=/var/lib/pg-node/xray-core|" "$ENV_FILE"
    colorized_echo green "✓ Xray-core $version installed."
    restart_command
}

uninstall_command() {
    check_root; detect_compose
    is_installed && compose_cmd down 2>/dev/null || true
    uninstall_node_script
    rm -rf "$APP_DIR"
    colorized_echo green "node uninstalled. Data kept at $DATA_DIR (remove manually if desired)."
}

completion_command() {
    check_root
    local f="/etc/bash_completion.d/$APP_NAME"
    cat >"$f" <<EOF
_pgnode(){ COMPREPLY=( \$(compgen -W "install update uninstall up down restart status logs core-update renew-cert edit edit-env install-script uninstall-script completion help" -- "\${COMP_WORDS[COMP_CWORD]}") ); }
complete -F _pgnode $APP_NAME
EOF
    colorized_echo green "✓ Bash completion installed: $f"
}

usage() {
    colorized_echo cyan "pg-node — multi-tenant node agent CLI"
    echo "Usage: $APP_NAME <command> [options]"
    echo
    echo "Commands:"
    echo "  install            Install & start the node agent"
    echo "  update             Rebuild & restart from latest sources"
    echo "  uninstall          Stop and remove the node"
    echo "  up | down | restart | status | logs"
    echo "  core-update [VER]  Install/replace Xray-core (default: latest)"
    echo "  renew-cert         Regenerate the self-signed certificate"
    echo "  edit | edit-env    Edit compose / .env"
    echo "  install-script     Install this CLI to /usr/local/bin"
    echo "  completion         Install bash completion"
    echo
    echo "Install options:"
    echo "  --master-key KEY   Master key (auto-generated if omitted)"
    echo "  --core-key KEY     Enable PasarGuard-compat gRPC (core config) with this key"
    echo "  --http-port PORT   HTTP control port (default 8090)"
    echo "  --grpc-port PORT   gRPC compat port (default 62050)"
    echo "  --san-entries CSV  Extra cert SANs (e.g. DNS:node.example.com)"
    echo "  --repo URL         Source repo to clone (default $REPO_URL)"
    echo "  -y, --yes          Non-interactive"
}

AUTO_CONFIRM="${AUTO_CONFIRM:-false}"
cmd="${1:-help}"; shift || true
case "$cmd" in
    install) install_command "$@" ;;
    update) update_command "$@" ;;
    uninstall) uninstall_command ;;
    up) up_command ;;
    down) down_command ;;
    restart) restart_command ;;
    status) status_command ;;
    logs) logs_command ;;
    core-update) core_update_command "$@" ;;
    renew-cert) renew_cert_command ;;
    edit) edit_command ;;
    edit-env) edit_env_command ;;
    install-script) check_root; SRC_DIR="$(pwd)"; install_node_script ;;
    uninstall-script) check_root; uninstall_node_script ;;
    completion) completion_command ;;
    help|-h|--help) usage ;;
    *) colorized_echo red "Unknown command: $cmd"; usage; exit 1 ;;
esac
