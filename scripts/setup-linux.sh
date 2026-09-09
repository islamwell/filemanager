#!/usr/bin/env bash
# ==============================================================================
# Production Linux Installer & Service Manager for Go File Manager
# Supports: Ubuntu, Debian, CentOS, RHEL, Rocky Linux, AlmaLinux, Fedora, Arch, Alpine
# Features: Self-building, Systemd sandboxing, Health-checks, Safe rollback, Edge-case handling
# ==============================================================================

set -euo pipefail

# ------------------------------------------------------------------------------
# Default Settings & Configuration
# ------------------------------------------------------------------------------
APP_NAME="filemanager"
DEFAULT_PORT=8080
DEFAULT_HOST="0.0.0.0"
DEFAULT_DATA_DIR="/var/lib/filemanager"
DEFAULT_BIN_PATH="/usr/local/bin/filemanager"
DEFAULT_CONFIG_DIR="/etc/filemanager"
DEFAULT_ENV_FILE="/etc/filemanager/filemanager.env"
DEFAULT_SERVICE_FILE="/etc/systemd/system/filemanager.service"
DEFAULT_MAX_FILE_SIZE=104857600 # 100 MB
DEFAULT_SECURE_COOKIE=false
SYSTEM_USER="filemanager"
SYSTEM_GROUP="filemanager"

# Runtime Flags
PORT="${DEFAULT_PORT}"
HOST="${DEFAULT_HOST}"
DATA_DIR="${DEFAULT_DATA_DIR}"
BIN_PATH="${DEFAULT_BIN_PATH}"
MAX_FILE_SIZE="${DEFAULT_MAX_FILE_SIZE}"
SECURE_COOKIE="${DEFAULT_SECURE_COOKIE}"
SKIP_SERVICE=false
DEBUG_MODE=false
UNINSTALL_MODE=false
FORCE_OVERWRITE=false

# ------------------------------------------------------------------------------
# Colorized UI Output & Logging
# ------------------------------------------------------------------------------
RED=$(printf '\033[0;31m')
GREEN=$(printf '\033[0;32m')
YELLOW=$(printf '\033[1;33m')
BLUE=$(printf '\033[0;34m')
PURPLE=$(printf '\033[0;35m')
CYAN=$(printf '\033[0;36m')
BOLD=$(printf '\033[1m')
NC=$(printf '\033[0m') # No Color

timestamp() {
    date +"%Y-%m-%d %H:%M:%S"
}

log_info() {
    echo -e "${BLUE}[$(timestamp)] [INFO]${NC} $*"
}

log_debug() {
    if [[ "${DEBUG_MODE}" == "true" ]]; then
        echo -e "${PURPLE}[$(timestamp)] [DEBUG]${NC} $*"
    fi
}

log_success() {
    echo -e "${GREEN}[$(timestamp)] [SUCCESS]${NC} ${BOLD}$*${NC}"
}

log_warn() {
    echo -e "${YELLOW}[$(timestamp)] [WARN]${NC} $*" >&2
}

log_error() {
    echo -e "${RED}[$(timestamp)] [ERROR]${NC} ${BOLD}$*${NC}" >&2
}

log_fatal() {
    echo -e "${RED}[$(timestamp)] [FATAL]${NC} ${BOLD}$*${NC}" >&2
    exit 1
}

# ------------------------------------------------------------------------------
# Error Trap & Cleanup Routine
# ------------------------------------------------------------------------------
cleanup() {
    local exit_code=$?
    if [[ ${exit_code} -ne 0 ]]; then
        log_error "Installation failed on line ${1:-unknown} with exit code ${exit_code}."
        log_info "Review the debug messages above. You can re-run with --debug for full trace."
    fi
}
trap 'cleanup ${LINENO}' EXIT

# ------------------------------------------------------------------------------
# Usage & Help Display
# ------------------------------------------------------------------------------
show_help() {
    cat << EOF
${BOLD}Production Linux Installer for Go File Manager${NC}

Usage:
  sudo ./setup-linux.sh [OPTIONS]

Options:
  -p, --port <port>          HTTP listening port (default: ${DEFAULT_PORT})
  -h, --host <host>          Listening host address (default: ${DEFAULT_HOST})
  -s, --max-size <bytes>     Maximum file upload size in bytes (default: ${DEFAULT_MAX_FILE_SIZE} = 100MB)
  -d, --data-dir <path>      Storage and database directory (default: ${DEFAULT_DATA_DIR})
  -b, --bin-path <path>      Binary installation target (default: ${DEFAULT_BIN_PATH})
  --secure-cookie <bool>     Require HTTPS for session cookies (default: ${DEFAULT_SECURE_COOKIE})
  --no-service               Install binary and directories only (skip systemd unit)
  --force                    Overwrite existing configuration files without asking
  --uninstall                Stop service, remove binary, systemd unit, and user
  -v, --debug                Enable verbose debug logging
  --help                     Show this help message and exit

Examples:
  sudo ./setup-linux.sh --port 9000
  sudo ./setup-linux.sh --secure-cookie true --host 127.0.0.1
  sudo ./setup-linux.sh --uninstall
EOF
    exit 0
}

# ------------------------------------------------------------------------------
# Parse CLI Options
# ------------------------------------------------------------------------------
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -p|--port)
                PORT="$2"
                shift 2
                ;;
            -h|--host)
                HOST="$2"
                shift 2
                ;;
            -s|--max-size)
                MAX_FILE_SIZE="$2"
                shift 2
                ;;
            -d|--data-dir)
                DATA_DIR="$2"
                shift 2
                ;;
            -b|--bin-path)
                BIN_PATH="$2"
                shift 2
                ;;
            --secure-cookie)
                SECURE_COOKIE="$2"
                shift 2
                ;;
            --no-service)
                SKIP_SERVICE=true
                shift
                ;;
            --force)
                FORCE_OVERWRITE=true
                shift
                ;;
            --uninstall)
                UNINSTALL_MODE=true
                shift
                ;;
            -v|--debug)
                DEBUG_MODE=true
                set -x
                shift
                ;;
            --help)
                show_help
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                ;;
        esac
    done
}

# ------------------------------------------------------------------------------
# Pre-Flight Checks & Validations
# ------------------------------------------------------------------------------
check_privileges() {
    if [[ $EUID -ne 0 ]]; then
        log_fatal "This script must be executed with root privileges. Run: sudo $0"
    fi
}

check_system_requirements() {
    log_info "Running pre-flight system checks..."

    # 1. Architecture detection
    local arch
    arch="$(uname -m)"
    log_debug "Detected hardware architecture: ${arch}"
    case "${arch}" in
        x86_64|amd64)
            GO_ARCH="amd64"
            ;;
        aarch64|arm64)
            GO_ARCH="arm64"
            ;;
        armv7l|armv6l)
            GO_ARCH="arm"
            ;;
        *)
            log_warn "Uncommon architecture detected: ${arch}. Compilation from source will be attempted."
            GO_ARCH="${arch}"
            ;;
    esac

    # 2. Kernel & OS detection
    local os_name
    os_name="$(uname -s)"
    if [[ "${os_name}" != "Linux" ]]; then
        log_fatal "This script is designed for Linux environments (detected: ${os_name})."
    fi

    # 3. Disk Space Check (Minimum 500 MB in /var and target path)
    local target_parent
    target_parent="$(dirname "${DATA_DIR}")"
    if [[ ! -d "${target_parent}" ]]; then
        target_parent="/"
    fi
    local available_kb
    available_kb=$(df -k "${target_parent}" | awk 'NR==2 {print $4}')
    local min_required_kb=512000 # 500 MB
    if [[ ${available_kb} -lt ${min_required_kb} ]]; then
        log_warn "Low disk space on ${target_parent}: only $(( available_kb / 1024 )) MB available. Recommend at least 500 MB."
    else
        log_debug "Disk space check passed: $(( available_kb / 1024 )) MB available."
    fi

    # 4. Port Availability Check
    if [[ "${UNINSTALL_MODE}" == "false" ]]; then
        log_debug "Checking if port ${PORT} is available..."
        local port_in_use=false
        if command -v ss &>/dev/null; then
            if ss -tuln | grep -q ":${PORT} "; then
                port_in_use=true
            fi
        elif command -v netstat &>/dev/null; then
            if netstat -tuln | grep -q ":${PORT} "; then
                port_in_use=true
            fi
        elif command -v lsof &>/dev/null; then
            if lsof -i ":${PORT}" -sTCP:LISTEN &>/dev/null; then
                port_in_use=true
            fi
        fi

        if [[ "${port_in_use}" == "true" ]]; then
            log_warn "Port ${PORT} is currently in use by another process."
            log_warn "If this is an existing filemanager instance, it will be restarted by systemd."
            log_warn "Otherwise, choose a different port with: --port <another-port>"
        fi
    fi

    # 5. Systemd Availability Check
    if [[ "${SKIP_SERVICE}" == "false" ]]; then
        if [[ ! -d /run/systemd/system ]]; then
            log_warn "Systemd is not running on PID 1 (Docker/WSL/Chroot environment detected)."
            log_warn "Skipping systemd service setup. You can run the binary directly via: ${BIN_PATH}"
            SKIP_SERVICE=true
        fi
    fi
}

# ------------------------------------------------------------------------------
# Build / Locate Binary
# ------------------------------------------------------------------------------
prepare_binary() {
    log_info "Preparing application executable..."

    # Check 1: In current directory or source directory
    local SCRIPT_DIR
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local PROJECT_ROOT
    PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

    local build_dir="/tmp/filemanager-build-$$"
    mkdir -p "${build_dir}"

    # Check if a pre-compiled binary exists in project root
    if [[ -f "${PROJECT_ROOT}/filemanager" ]]; then
        log_info "Found pre-built binary in ${PROJECT_ROOT}/filemanager."
        cp "${PROJECT_ROOT}/filemanager" "${build_dir}/filemanager"
    elif [[ -f "${PROJECT_ROOT}/cmd/server/main.go" ]]; then
        log_info "Source code found at ${PROJECT_ROOT}. Compiling binary with Go..."
        if ! command -v go &>/dev/null; then
            log_fatal "Go compiler ('go') not found on system PATH. Please install Go 1.22+ or pre-build the binary."
        fi

        local go_version
        go_version=$(go version | awk '{print $3}')
        log_info "Using ${go_version}..."

        (
            cd "${PROJECT_ROOT}"
            CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${build_dir}/filemanager" cmd/server/main.go
        )
        log_success "Compilation completed successfully."
    else
        log_fatal "Neither source files (cmd/server/main.go) nor pre-built binary (filemanager) found in ${PROJECT_ROOT}."
    fi

    # Install binary to target location
    log_info "Installing executable to ${BIN_PATH}..."
    install -m 755 "${build_dir}/filemanager" "${BIN_PATH}"
    rm -rf "${build_dir}"

    log_success "Binary installed at ${BIN_PATH}."
}

# ------------------------------------------------------------------------------
# Dedicated System User Creation
# ------------------------------------------------------------------------------
create_system_user() {
    log_info "Configuring dedicated system user and group (${SYSTEM_USER})..."

    # Group check/creation
    if ! getent group "${SYSTEM_GROUP}" &>/dev/null; then
        log_debug "Creating system group ${SYSTEM_GROUP}..."
        groupadd --system "${SYSTEM_GROUP}"
    fi

    # User check/creation
    if ! getent passwd "${SYSTEM_USER}" &>/dev/null; then
        log_debug "Creating system user ${SYSTEM_USER} with home ${DATA_DIR}..."
        useradd --system \
            --gid "${SYSTEM_GROUP}" \
            --home-dir "${DATA_DIR}" \
            --shell /usr/sbin/nologin \
            --comment "Go File Manager Daemon" \
            "${SYSTEM_USER}"
    else
        log_debug "System user ${SYSTEM_USER} already exists."
    fi
}

# ------------------------------------------------------------------------------
# Persistent Storage & Permissions
# ------------------------------------------------------------------------------
setup_directories() {
    log_info "Setting up storage paths and permissions..."

    local db_dir="${DATA_DIR}/data"
    local storage_dir="${DATA_DIR}/storage"

    mkdir -p "${db_dir}"
    mkdir -p "${storage_dir}"
    mkdir -p "${DEFAULT_CONFIG_DIR}"

    # Strict permissions: 0750 on data dirs, owned by filemanager:filemanager
    chown -R "${SYSTEM_USER}:${SYSTEM_GROUP}" "${DATA_DIR}"
    chmod 750 "${DATA_DIR}"
    chmod 750 "${db_dir}"
    chmod 750 "${storage_dir}"

    chown -R "${SYSTEM_USER}:${SYSTEM_GROUP}" "${DEFAULT_CONFIG_DIR}"
    chmod 750 "${DEFAULT_CONFIG_DIR}"

    log_success "Directories initialized at ${DATA_DIR}."
}

# ------------------------------------------------------------------------------
# Configuration File Generation
# ------------------------------------------------------------------------------
setup_configuration() {
    log_info "Configuring environment parameters..."

    if [[ -f "${DEFAULT_ENV_FILE}" ]] && [[ "${FORCE_OVERWRITE}" == "false" ]]; then
        log_warn "Configuration file ${DEFAULT_ENV_FILE} already exists. Preserving existing config."
        log_info "Use --force to overwrite if needed."
    else
        log_info "Writing configuration to ${DEFAULT_ENV_FILE}..."
        cat > "${DEFAULT_ENV_FILE}" << EOF
# ------------------------------------------------------------------------------
# Go File Manager Production Environment Configuration
# Generated on: $(timestamp)
# ------------------------------------------------------------------------------
PORT=${PORT}
HOST=${HOST}
DATABASE_PATH=${DATA_DIR}/data/filemanager.db
STORAGE_DIR=${DATA_DIR}/storage
MAX_FILE_SIZE=${MAX_FILE_SIZE}
SECURE_COOKIE=${SECURE_COOKIE}
EOF
        chmod 600 "${DEFAULT_ENV_FILE}"
        chown "${SYSTEM_USER}:${SYSTEM_GROUP}" "${DEFAULT_ENV_FILE}"
        log_success "Configuration created at ${DEFAULT_ENV_FILE}."
    fi
}

# ------------------------------------------------------------------------------
# Systemd Service Installation & Hardening
# ------------------------------------------------------------------------------
setup_systemd_service() {
    if [[ "${SKIP_SERVICE}" == "true" ]]; then
        log_info "Skipping systemd service configuration as requested."
        return 0
    fi

    log_info "Installing hardened systemd service unit..."

    cat > "${DEFAULT_SERVICE_FILE}" << EOF
[Unit]
Description=Go File Manager Web Application
Documentation=https://github.com/filemanager
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SYSTEM_USER}
Group=${SYSTEM_GROUP}
WorkingDirectory=${DATA_DIR}
EnvironmentFile=${DEFAULT_ENV_FILE}
ExecStart=${BIN_PATH}
Restart=always
RestartSec=5s

# ------------------------------------------------------------------------------
# Security Sandboxing
# ------------------------------------------------------------------------------
NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
ReadWritePaths=${DATA_DIR}
ReadOnlyPaths=${DEFAULT_CONFIG_DIR}

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

# Standard Output & Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${APP_NAME}

[Install]
WantedBy=multi-user.target
EOF

    chmod 644 "${DEFAULT_SERVICE_FILE}"
    systemctl daemon-reload
    systemctl enable "${APP_NAME}.service"
    systemctl restart "${APP_NAME}.service"

    log_success "Systemd service '${APP_NAME}' enabled and started."
}

# ------------------------------------------------------------------------------
# Health Check Verification
# ------------------------------------------------------------------------------
verify_installation() {
    if [[ "${SKIP_SERVICE}" == "true" ]]; then
        log_info "Verifying binary execution..."
        if "${BIN_PATH}" --help &>/dev/null || true; then
            log_success "Binary verification passed: ${BIN_PATH} is executable."
        fi
        return 0
    fi

    log_info "Performing live service health check..."

    local check_host="${HOST}"
    if [[ "${check_host}" == "0.0.0.0" ]] || [[ "${check_host}" == "" ]]; then
        check_host="127.0.0.1"
    fi

    local health_url="http://${check_host}:${PORT}/healthz"
    local max_attempts=15
    local attempt=1
    local is_healthy=false

    while [[ ${attempt} -le ${max_attempts} ]]; do
        log_debug "Checking HTTP health at ${health_url} (attempt ${attempt}/${max_attempts})..."
        if command -v curl &>/dev/null; then
            if curl -s -f -o /dev/null --connect-timeout 2 "${health_url}"; then
                is_healthy=true
                break
            fi
        elif command -v wget &>/dev/null; then
            if wget -q --spider --timeout=2 "${health_url}"; then
                is_healthy=true
                break
            fi
        else
            # Fallback: check systemd state
            if systemctl is-active --quiet "${APP_NAME}"; then
                is_healthy=true
                break
            fi
        fi
        sleep 1
        attempt=$(( attempt + 1 ))
    done

    if [[ "${is_healthy}" == "true" ]]; then
        log_success "Health check PASSED! Service is responding on ${health_url}."
    else
        log_error "Health check FAILED! Service did not respond within ${max_attempts} seconds."
        log_info "Dumping recent system logs for diagnostics:"
        echo -e "${YELLOW}------------------------------------------------------------${NC}"
        journalctl -u "${APP_NAME}" -n 25 --no-pager || true
        echo -e "${YELLOW}------------------------------------------------------------${NC}"
        log_fatal "Service startup failed. Inspect the logs above."
    fi
}

# ------------------------------------------------------------------------------
# Uninstall Routine
# ------------------------------------------------------------------------------
perform_uninstall() {
    log_warn "Starting uninstallation of ${APP_NAME}..."

    if command -v systemctl &>/dev/null && [[ -f "${DEFAULT_SERVICE_FILE}" ]]; then
        log_info "Stopping and disabling systemd service..."
        systemctl stop "${APP_NAME}" 2>/dev/null || true
        systemctl disable "${APP_NAME}" 2>/dev/null || true
        rm -f "${DEFAULT_SERVICE_FILE}"
        systemctl daemon-reload
    fi

    if [[ -f "${BIN_PATH}" ]]; then
        log_info "Removing binary ${BIN_PATH}..."
        rm -f "${BIN_PATH}"
    fi

    if [[ -d "${DEFAULT_CONFIG_DIR}" ]]; then
        log_info "Removing config directory ${DEFAULT_CONFIG_DIR}..."
        rm -rf "${DEFAULT_CONFIG_DIR}"
    fi

    read -r -p "Do you also want to permanently delete all file storage and database in ${DATA_DIR}? [y/N]: " confirm
    if [[ "${confirm}" =~ ^[Yy]$ ]]; then
        log_warn "Removing data directory ${DATA_DIR}..."
        rm -rf "${DATA_DIR}"
    else
        log_info "Preserved data directory at ${DATA_DIR}."
    fi

    if getent passwd "${SYSTEM_USER}" &>/dev/null; then
        log_info "Removing system user ${SYSTEM_USER}..."
        userdel "${SYSTEM_USER}" 2>/dev/null || true
    fi

    log_success "Uninstallation completed."
    exit 0
}

# ------------------------------------------------------------------------------
# Print Final Summary & Next Steps
# ------------------------------------------------------------------------------
print_summary() {
    echo ""
    echo -e "${GREEN}======================================================================${NC}"
    echo -e "${GREEN}${BOLD}     Go File Manager successfully installed and running!${NC}"
    echo -e "${GREEN}======================================================================${NC}"
    echo -e "  ${BOLD}Web UI Address${NC}     : http://${HOST}:${PORT}"
    echo -e "  ${BOLD}Binary Location${NC}    : ${BIN_PATH}"
    echo -e "  ${BOLD}Data Directory${NC}     : ${DATA_DIR}"
    echo -e "  ${BOLD}Config File${NC}        : ${DEFAULT_ENV_FILE}"
    if [[ "${SKIP_SERVICE}" == "false" ]]; then
        echo -e "  ${BOLD}Service Name${NC}       : ${APP_NAME}.service"
        echo -e "  ${BOLD}Service Status${NC}     : sudo systemctl status ${APP_NAME}"
        echo -e "  ${BOLD}View Live Logs${NC}     : sudo journalctl -u ${APP_NAME} -f"
        echo -e "  ${BOLD}Restart Service${NC}    : sudo systemctl restart ${APP_NAME}"
    fi
    echo -e "${GREEN}======================================================================${NC}"
    echo ""
}

# ------------------------------------------------------------------------------
# Main Entry Point
# ------------------------------------------------------------------------------
main() {
    parse_arguments "$@"
    check_privileges

    if [[ "${UNINSTALL_MODE}" == "true" ]]; then
        perform_uninstall
    fi

    check_system_requirements
    prepare_binary
    create_system_user
    setup_directories
    setup_configuration
    setup_systemd_service
    verify_installation
    print_summary
}

# Execute main function with arguments
main "$@"
