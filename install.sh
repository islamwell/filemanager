#!/usr/bin/env bash
# ==============================================================================
# Go File Manager - All-In-One Linux Production Installer & Management Tool
# Wizard, Interactive Menu, Express Setup, Plesk Integration, Backups & Systemd
# ==============================================================================

set -euo pipefail

# ------------------------------------------------------------------------------
# Default Settings
# ------------------------------------------------------------------------------
APP_NAME="filemanager"
DEFAULT_PORT=8080
DEFAULT_HOST="0.0.0.0"
DEFAULT_DATA_DIR="/var/lib/filemanager"
DEFAULT_BIN_PATH="/usr/local/bin/filemanager"
DEFAULT_CONFIG_DIR="/etc/filemanager"
DEFAULT_ENV_FILE="/etc/filemanager/filemanager.env"
DEFAULT_SERVICE_FILE="/etc/systemd/system/filemanager.service"
DEFAULT_BACKUP_DIR="/var/backups/filemanager"
DEFAULT_MAX_FILE_MB=100
SYSTEM_USER="filemanager"
SYSTEM_GROUP="filemanager"

# Runtime Variables
PORT="${DEFAULT_PORT}"
HOST="${DEFAULT_HOST}"
DATA_DIR="${DEFAULT_DATA_DIR}"
BIN_PATH="${DEFAULT_BIN_PATH}"
MAX_FILE_MB="${DEFAULT_MAX_FILE_MB}"
SECURE_COOKIE=false
SKIP_SERVICE=false
DEBUG_MODE=false
FORCE_OVERWRITE=false
INTERACTIVE_MODE=true

# ------------------------------------------------------------------------------
# Color Constants & Terminal Styling
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
# Error Handler
# ------------------------------------------------------------------------------
cleanup() {
    local exit_code=$?
    if [[ ${exit_code} -ne 0 ]]; then
        log_error "Operation terminated on line ${1:-unknown} with exit code ${exit_code}."
    fi
}
trap 'cleanup ${LINENO}' EXIT

# ------------------------------------------------------------------------------
# UI Banner
# ------------------------------------------------------------------------------
print_banner() {
    clear 2>/dev/null || true
    echo -e "${CYAN}${BOLD}"
    cat << "EOF"
  ███████╗██╗██╗     ███████╗███╗   ███╗ █████╗ ███╗   ██╗ █████╗  ██████╗ ███████╗██████╗ 
  ██╔════╝██║██║     ██╔════╝████╗ ████║██╔══██╗████╗  ██║██╔══██╗██╔════╝ ██╔════╝██╔══██╗
  █████╗  ██║██║     █████╗  ██╔████╔██║███████║██╔██╗ ██║███████║██║  ███╗█████╗  ██████╔╝
  ██╔══╝  ██║██║     ██╔══╝  ██║╚██╔╝██║██╔══██║██║╚██╗██║██╔══██║██║   ██║██╔══╝  ██╔══██╗
  ██║     ██║███████╗███████╗██║ ╚═╝ ██║██║  ██║██║ ╚████║██║  ██║╚██████╔╝███████╗██║  ██║
  ╚═╝     ╚═╝╚══════╝╚══════╝╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═╝
EOF
    echo -e "${NC}"
    echo -e "  ${BOLD}Go File Manager — Production A-to-Z Linux Installer & Management Suite${NC}"
    echo -e "  ${PURPLE}Pure Go (CGO-free SQLite) • MP3 Range Player • 4-Level Folders • Fuzzy Search${NC}"
    echo -e "  ----------------------------------------------------------------------------------"
    echo ""
}

# ------------------------------------------------------------------------------
# Privilege & Environment Checks
# ------------------------------------------------------------------------------
check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_fatal "This installer requires root privileges. Please run: sudo $0"
    fi
}

check_system_requirements() {
    log_debug "Running system checks..."
    
    local os_name
    os_name="$(uname -s)"
    if [[ "${os_name}" != "Linux" ]]; then
        log_fatal "This installer is designed for Linux (detected: ${os_name})."
    fi

    # Check available disk space (minimum 500 MB)
    local target_parent
    target_parent="$(dirname "${DATA_DIR}")"
    if [[ ! -d "${target_parent}" ]]; then target_parent="/"; fi
    local available_kb
    available_kb=$(df -k "${target_parent}" 2>/dev/null | awk 'NR==2 {print $4}' || echo "1048576")
    if [[ ${available_kb} -lt 512000 ]]; then
        log_warn "Low disk space on ${target_parent}: only $(( available_kb / 1024 )) MB available."
    fi

    # Systemd check
    if [[ "${SKIP_SERVICE}" == "false" ]] && [[ ! -d /run/systemd/system ]]; then
        log_warn "Systemd is not running as PID 1 (Docker/WSL detected). Skipping service unit."
        SKIP_SERVICE=true
    fi
}

# ------------------------------------------------------------------------------
# Port Collision Check
# ------------------------------------------------------------------------------
is_port_in_use() {
    local port_to_check="$1"
    if command -v ss &>/dev/null; then
        ss -tuln | grep -q ":${port_to_check} " && return 0
    elif command -v netstat &>/dev/null; then
        netstat -tuln | grep -q ":${port_to_check} " && return 0
    elif command -v lsof &>/dev/null; then
        lsof -i ":${port_to_check}" -sTCP:LISTEN &>/dev/null && return 0
    fi
    return 1
}

# ------------------------------------------------------------------------------
# Binary Preparation (Pre-built or Source Compilation)
# ------------------------------------------------------------------------------
prepare_binary() {
    log_info "Preparing application binary..."
    local SCRIPT_DIR
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    local build_tmp="/tmp/filemanager-install-$$"
    mkdir -p "${build_tmp}"

    # Search paths for pre-built binary
    if [[ -f "${SCRIPT_DIR}/filemanager" ]]; then
        log_info "Found pre-built binary in current directory."
        cp "${SCRIPT_DIR}/filemanager" "${build_tmp}/filemanager"
    elif [[ -f "${SCRIPT_DIR}/cmd/server/main.go" ]]; then
        log_info "Compiling binary from Go source files..."
        if ! command -v go &>/dev/null; then
            log_warn "Go compiler not found. Attempting to install Go or build dependencies..."
            if command -v apt-get &>/dev/null; then
                apt-get update -qq && apt-get install -y -qq golang
            elif command -v dnf &>/dev/null; then
                dnf install -y -q golang
            elif command -v yum &>/dev/null; then
                yum install -y -q golang
            elif command -v apk &>/dev/null; then
                apk add --no-cache go
            fi
        fi

        if ! command -v go &>/dev/null; then
            log_fatal "Could not find or install 'go'. Please install Go 1.22+ or copy the pre-built 'filemanager' binary here."
        fi

        (
            cd "${SCRIPT_DIR}"
            CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${build_tmp}/filemanager" cmd/server/main.go
        )
        log_success "Go compilation completed."
    else
        log_fatal "Cannot find 'filemanager' binary or 'cmd/server/main.go' in ${SCRIPT_DIR}."
    fi

    # Install binary
    log_info "Installing executable to ${BIN_PATH}..."
    install -m 755 "${build_tmp}/filemanager" "${BIN_PATH}"
    rm -rf "${build_tmp}"
    log_success "Binary installed at ${BIN_PATH}."
}

# ------------------------------------------------------------------------------
# System User & Storage Layout
# ------------------------------------------------------------------------------
setup_user_and_storage() {
    log_info "Setting up dedicated system user and storage directories..."

    if ! getent group "${SYSTEM_GROUP}" &>/dev/null; then
        groupadd --system "${SYSTEM_GROUP}"
    fi

    if ! getent passwd "${SYSTEM_USER}" &>/dev/null; then
        useradd --system \
            --gid "${SYSTEM_GROUP}" \
            --home-dir "${DATA_DIR}" \
            --shell /usr/sbin/nologin \
            --comment "Go File Manager Daemon" \
            "${SYSTEM_USER}"
    fi

    mkdir -p "${DATA_DIR}/data"
    mkdir -p "${DATA_DIR}/storage"
    mkdir -p "${DEFAULT_CONFIG_DIR}"
    mkdir -p "${DEFAULT_BACKUP_DIR}"

    chown -R "${SYSTEM_USER}:${SYSTEM_GROUP}" "${DATA_DIR}"
    chmod -R 750 "${DATA_DIR}"
    chown -R "${SYSTEM_USER}:${SYSTEM_GROUP}" "${DEFAULT_CONFIG_DIR}"
    chmod 750 "${DEFAULT_CONFIG_DIR}"

    log_success "Directories configured with 0750 permissions at ${DATA_DIR}."
}

# ------------------------------------------------------------------------------
# Configuration Generation
# ------------------------------------------------------------------------------
write_configuration() {
    log_info "Writing configuration to ${DEFAULT_ENV_FILE}..."
    local max_bytes=$(( MAX_FILE_MB * 1024 * 1024 ))

    if [[ -f "${DEFAULT_ENV_FILE}" ]] && [[ "${FORCE_OVERWRITE}" == "false" ]]; then
        log_warn "Config file already exists. Backing up to ${DEFAULT_ENV_FILE}.bak..."
        cp "${DEFAULT_ENV_FILE}" "${DEFAULT_ENV_FILE}.bak.$(date +%s)"
    fi

    cat > "${DEFAULT_ENV_FILE}" << EOF
# ------------------------------------------------------------------------------
# Go File Manager Configuration
# Generated on: $(timestamp)
# ------------------------------------------------------------------------------
PORT=${PORT}
HOST=${HOST}
DATABASE_PATH=${DATA_DIR}/data/filemanager.db
STORAGE_DIR=${DATA_DIR}/storage
MAX_FILE_SIZE=${max_bytes}
SECURE_COOKIE=${SECURE_COOKIE}
EOF

    chmod 600 "${DEFAULT_ENV_FILE}"
    chown "${SYSTEM_USER}:${SYSTEM_GROUP}" "${DEFAULT_ENV_FILE}"
    log_success "Configuration created at ${DEFAULT_ENV_FILE}."
}

# ------------------------------------------------------------------------------
# Systemd Service Unit
# ------------------------------------------------------------------------------
install_systemd_service() {
    if [[ "${SKIP_SERVICE}" == "true" ]]; then
        log_info "Skipping systemd service installation."
        return 0
    fi

    log_info "Configuring hardened systemd service unit..."

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

# Security Sandboxing Directives
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
# Live Health Check Loop
# ------------------------------------------------------------------------------
run_health_check() {
    if [[ "${SKIP_SERVICE}" == "true" ]]; then
        return 0
    fi

    log_info "Running live health check on http://127.0.0.1:${PORT}/healthz..."
    local max_attempts=15
    local attempt=1
    local is_healthy=false

    while [[ ${attempt} -le ${max_attempts} ]]; do
        if command -v curl &>/dev/null; then
            if curl -s -f -o /dev/null --connect-timeout 2 "http://127.0.0.1:${PORT}/healthz" || curl -s -f -o /dev/null --connect-timeout 2 "http://127.0.0.1:${PORT}/login"; then
                is_healthy=true
                break
            fi
        elif command -v wget &>/dev/null; then
            if wget -q --spider --timeout=2 "http://127.0.0.1:${PORT}/healthz" || wget -q --spider --timeout=2 "http://127.0.0.1:${PORT}/login"; then
                is_healthy=true
                break
            fi
        elif systemctl is-active --quiet "${APP_NAME}"; then
            is_healthy=true
            break
        fi
        sleep 1
        attempt=$(( attempt + 1 ))
    done

    if [[ "${is_healthy}" == "true" ]]; then
        log_success "Health check PASSED! Application is actively serving requests."
    else
        log_error "Health check FAILED! Service did not respond within ${max_attempts}s."
        echo -e "${YELLOW}Recent Journal Logs:${NC}"
        journalctl -u "${APP_NAME}" -n 25 --no-pager || true
        log_fatal "Service failed to start properly. Inspect logs above."
    fi
}

# ------------------------------------------------------------------------------
# Core Installation Runner
# ------------------------------------------------------------------------------
execute_installation() {
    check_root
    check_system_requirements

    if is_port_in_use "${PORT}"; then
        log_warn "Port ${PORT} is currently listening. If this is an existing filemanager instance, it will be safely restarted."
    fi

    prepare_binary
    setup_user_and_storage
    write_configuration
    install_systemd_service
    run_health_check

    print_install_summary
}

# ------------------------------------------------------------------------------
# Summary Output
# ------------------------------------------------------------------------------
print_install_summary() {
    echo ""
    echo -e "${GREEN}======================================================================${NC}"
    echo -e "${GREEN}${BOLD}     Go File Manager successfully installed and running!${NC}"
    echo -e "${GREEN}======================================================================${NC}"
    echo -e "  ${BOLD}Listening Address${NC}  : http://${HOST}:${PORT}"
    echo -e "  ${BOLD}Binary Location${NC}    : ${BIN_PATH}"
    echo -e "  ${BOLD}Data Directory${NC}     : ${DATA_DIR}"
    echo -e "  ${BOLD}Config File${NC}        : ${DEFAULT_ENV_FILE}"
    if [[ "${SKIP_SERVICE}" == "false" ]]; then
        echo -e "  ${BOLD}Service Status${NC}     : sudo systemctl status ${APP_NAME}"
        echo -e "  ${BOLD}Live Log Stream${NC}    : sudo journalctl -u ${APP_NAME} -f"
        echo -e "  ${BOLD}Restart Service${NC}    : sudo systemctl restart ${APP_NAME}"
    fi
    echo -e "${GREEN}======================================================================${NC}"
    echo ""
}

# ------------------------------------------------------------------------------
# Interactive Wizard Mode
# ------------------------------------------------------------------------------
run_wizard() {
    print_banner
    echo -e "${BOLD}${CYAN}=== Step-by-Step Installation Wizard ===${NC}\n"

    # 1. Profile selection
    echo -e "${BOLD}Select Deployment Mode:${NC}"
    echo "  1) Standalone Direct HTTP (Listens publicly on 0.0.0.0)"
    echo "  2) Behind Reverse Proxy / Plesk / Nginx / Caddy (Listens on 127.0.0.1 with Secure HTTPS cookies)"
    echo "  3) Custom advanced settings"
    read -r -p "Select [1-3, default: 1]: " profile_choice
    profile_choice="${profile_choice:-1}"

    case "${profile_choice}" in
        1)
            HOST="0.0.0.0"
            SECURE_COOKIE=false
            ;;
        2)
            HOST="127.0.0.1"
            SECURE_COOKIE=true
            ;;
        3)
            read -r -p "Enter bind host [default: ${DEFAULT_HOST}]: " custom_host
            HOST="${custom_host:-${DEFAULT_HOST}}"
            read -r -p "Enable Secure HTTPS Cookies? (y/N): " custom_sec
            if [[ "${custom_sec}" =~ ^[Yy]$ ]]; then SECURE_COOKIE=true; else SECURE_COOKIE=false; fi
            ;;
        *)
            HOST="0.0.0.0"
            SECURE_COOKIE=false
            ;;
    esac

    # 2. Port selection
    read -r -p "Enter HTTP port [default: ${DEFAULT_PORT}]: " custom_port
    PORT="${custom_port:-${DEFAULT_PORT}}"

    # 3. Max upload size
    read -r -p "Enter Maximum upload size in MB [default: ${DEFAULT_MAX_FILE_MB}]: " custom_size
    MAX_FILE_MB="${custom_size:-${DEFAULT_MAX_FILE_MB}}"

    # 4. Storage directory
    read -r -p "Enter Persistent storage path [default: ${DEFAULT_DATA_DIR}]: " custom_data
    DATA_DIR="${custom_data:-${DEFAULT_DATA_DIR}}"

    # Summary confirmation
    echo ""
    echo -e "${YELLOW}------------------------------------------------------------${NC}"
    echo -e "${BOLD}Configuration Summary:${NC}"
    echo -e "  Host           : ${HOST}"
    echo -e "  Port           : ${PORT}"
    echo -e "  Storage Path   : ${DATA_DIR}"
    echo -e "  Max File Size  : ${MAX_FILE_MB} MB"
    echo -e "  Secure Cookie  : ${SECURE_COOKIE}"
    echo -e "${YELLOW}------------------------------------------------------------${NC}"
    read -r -p "Proceed with installation? [Y/n]: " proceed
    if [[ "${proceed}" =~ ^[Nn]$ ]]; then
        echo "Installation aborted."
        exit 0
    fi

    FORCE_OVERWRITE=true
    execute_installation
}

# ------------------------------------------------------------------------------
# Plesk Preset Mode
# ------------------------------------------------------------------------------
run_plesk_preset() {
    print_banner
    echo -e "${CYAN}${BOLD}=== Plesk / Nginx Reverse Proxy Setup Preset ===${NC}\n"
    HOST="127.0.0.1"
    PORT="8080"
    SECURE_COOKIE=true
    FORCE_OVERWRITE=true

    log_info "Configuring for Plesk (127.0.0.1:8080, HTTPS Secure Cookies)..."
    execute_installation

    echo -e "${CYAN}${BOLD}======================================================================${NC}"
    echo -e "${BOLD}📋 Paste this into your Plesk Domain 'Additional nginx directives':${NC}"
    echo -e "${CYAN}----------------------------------------------------------------------${NC}"
    cat << EOF
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host \$host;
    proxy_set_header X-Real-IP \$remote_addr;
    proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto \$scheme;

    # High speed streaming for 100MB+ uploads & byte-range MP3 audio
    proxy_request_buffering off;
    proxy_buffering off;
    client_max_body_size ${MAX_FILE_MB}M;
}
EOF
    echo -e "${CYAN}======================================================================${NC}\n"
}

# ------------------------------------------------------------------------------
# Backup Routine
# ------------------------------------------------------------------------------
perform_backup() {
    check_root
    log_info "Initiating live backup of Go File Manager..."
    mkdir -p "${DEFAULT_BACKUP_DIR}"

    local backup_file="${DEFAULT_BACKUP_DIR}/filemanager-backup-$(date +%Y%m%d-%H%M%S).tar.gz"
    
    if [[ ! -d "${DATA_DIR}" ]]; then
        log_fatal "Data directory ${DATA_DIR} does not exist. Nothing to backup."
    fi

    # Create compressed archive
    tar -czf "${backup_file}" -C "${DATA_DIR}" data storage
    chmod 600 "${backup_file}"

    local backup_size
    backup_size=$(du -h "${backup_file}" | awk '{print $1}')
    log_success "Backup completed successfully!"
    echo -e "  ${BOLD}Archive Path${NC} : ${backup_file}"
    echo -e "  ${BOLD}Archive Size${NC} : ${backup_size}"
    echo ""
}

# ------------------------------------------------------------------------------
# Service Status & Logs Viewer
# ------------------------------------------------------------------------------
view_status_and_logs() {
    check_root
    print_banner
    echo -e "${BOLD}--- Service Status ---${NC}"
    systemctl status "${APP_NAME}" --no-pager || true
    echo ""
    echo -e "${BOLD}--- Last 20 Log Lines ---${NC}"
    journalctl -u "${APP_NAME}" -n 20 --no-pager || true
    echo ""
    read -r -p "Press [Enter] to return to the menu..." _
}

# ------------------------------------------------------------------------------
# View / Edit Configuration
# ------------------------------------------------------------------------------
manage_config() {
    check_root
    if [[ -f "${DEFAULT_ENV_FILE}" ]]; then
        echo -e "${CYAN}${BOLD}Current Configuration (${DEFAULT_ENV_FILE}):${NC}"
        echo -e "${YELLOW}------------------------------------------------------------${NC}"
        cat "${DEFAULT_ENV_FILE}"
        echo -e "${YELLOW}------------------------------------------------------------${NC}"
        echo ""
        read -r -p "Open in nano editor? [y/N]: " edit_choice
        if [[ "${edit_choice}" =~ ^[Yy]$ ]]; then
            ${EDITOR:-nano} "${DEFAULT_ENV_FILE}"
            systemctl restart "${APP_NAME}" 2>/dev/null || true
            log_success "Service restarted with updated configuration."
        fi
    else
        log_warn "No configuration file found at ${DEFAULT_ENV_FILE}."
    fi
}

# ------------------------------------------------------------------------------
# Uninstallation
# ------------------------------------------------------------------------------
perform_uninstall() {
    check_root
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

    log_success "Uninstallation complete."
    exit 0
}

# ------------------------------------------------------------------------------
# Interactive Menu Loop
# ------------------------------------------------------------------------------
main_menu() {
    while true; do
        print_banner
        echo -e "${BOLD}Select an action:${NC}\n"
        echo -e "  ${GREEN}${BOLD}1)${NC} 🧙 Step-by-Step Installation Wizard ${GREEN}(Recommended)${NC}"
        echo -e "  ${CYAN}${BOLD}2)${NC} ⚡ Express 1-Click Install (Default Production Settings)"
        echo -e "  ${PURPLE}${BOLD}3)${NC} 🌐 Plesk / Nginx Reverse-Proxy Profile"
        echo -e "  ${BLUE}${BOLD}4)${NC} 🔄 Update / Rebuild Binary (Zero Data Loss)"
        echo -e "  ${YELLOW}${BOLD}5)${NC} 📊 Check Service Status & Recent Logs"
        echo -e "  ${CYAN}${BOLD}6)${NC} ⚙️ View / Edit Configuration"
        echo -e "  ${GREEN}${BOLD}7)${NC} 💾 Backup Database & Storage"
        echo -e "  ${RED}${BOLD}8)${NC} 🗑️ Uninstall Service & Application"
        echo -e "  ${BOLD}0)${NC} ❌ Exit"
        echo ""
        read -r -p "Enter choice [0-8]: " choice

        case "${choice}" in
            1)
                run_wizard
                read -r -p "Press [Enter] to continue..." _
                ;;
            2)
                PORT=8080
                HOST="0.0.0.0"
                SECURE_COOKIE=false
                FORCE_OVERWRITE=false
                execute_installation
                read -r -p "Press [Enter] to continue..." _
                ;;
            3)
                run_plesk_preset
                read -r -p "Press [Enter] to continue..." _
                ;;
            4)
                check_root
                log_info "Rebuilding binary and updating..."
                prepare_binary
                systemctl restart "${APP_NAME}" 2>/dev/null || true
                log_success "Application updated and service restarted successfully."
                read -r -p "Press [Enter] to continue..." _
                ;;
            5)
                view_status_and_logs
                ;;
            6)
                manage_config
                read -r -p "Press [Enter] to continue..." _
                ;;
            7)
                perform_backup
                read -r -p "Press [Enter] to continue..." _
                ;;
            8)
                perform_uninstall
                ;;
            0|q|Q)
                echo "Exiting."
                exit 0
                ;;
            *)
                echo -e "${RED}Invalid selection.${NC}"
                sleep 1
                ;;
        esac
    done
}

# ------------------------------------------------------------------------------
# CLI Flag Parser
# ------------------------------------------------------------------------------
parse_cli_flags() {
    if [[ $# -eq 0 ]]; then
        # If no flags and running in interactive terminal, launch menu
        if [[ -t 0 ]]; then
            main_menu
            exit 0
        else
            # Non-interactive fallback
            execute_installation
            exit 0
        fi
    fi

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --wizard)
                run_wizard
                exit 0
                ;;
            --express)
                execute_installation
                exit 0
                ;;
            --plesk)
                run_plesk_preset
                exit 0
                ;;
            --backup)
                perform_backup
                exit 0
                ;;
            --status)
                view_status_and_logs
                exit 0
                ;;
            --uninstall)
                perform_uninstall
                exit 0
                ;;
            -p|--port)
                PORT="$2"
                shift 2
                ;;
            -h|--host)
                HOST="$2"
                shift 2
                ;;
            -s|--max-size-mb)
                MAX_FILE_MB="$2"
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
            -v|--debug)
                DEBUG_MODE=true
                set -x
                shift
                ;;
            --help)
                print_banner
                echo "Usage:"
                echo "  Interactive Menu : sudo ./install.sh"
                echo "  Step Wizard      : sudo ./install.sh --wizard"
                echo "  Express Setup    : sudo ./install.sh --express"
                echo "  Plesk Preset     : sudo ./install.sh --plesk"
                echo "  Backup Archive   : sudo ./install.sh --backup"
                echo "  Uninstall        : sudo ./install.sh --uninstall"
                echo "  Custom Flags     : sudo ./install.sh --port 9000 --host 127.0.0.1"
                exit 0
                ;;
            *)
                log_error "Unknown flag: $1"
                echo "Run: ./install.sh --help"
                exit 1
                ;;
        esac
    done

    execute_installation
}

# Entrypoint
parse_cli_flags "$@"
