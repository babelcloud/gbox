#!/bin/bash

# GBOX Installation Script
# This script installs GBOX CLI and its dependencies
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/babelcloud/gbox/main/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/babelcloud/gbox/main/install.sh | bash -s -- -y --with-deps --update

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
DIM='\033[2m' # Dim/faint text
NC='\033[0m'  # No Color
BOLD='\033[1m'

# Non-interactive mode flag
NON_INTERACTIVE=false

# Install dependencies flag
WITH_DEPS=false

# Update CLI flag (empty means not specified, true means update, false means no update)
UPDATE_CLI=""

# Detect OS
OS="$(uname -s)"
ARCH="$(uname -m)"

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
        -y | --yes | --non-interactive)
            NON_INTERACTIVE=true
            shift
            ;;
        --with-deps)
            WITH_DEPS=true
            shift
            ;;
        --update)
            # Check if next argument is a value (false/true)
            if [[ $# -gt 1 ]] && [[ "$2" == "false" || "$2" == "true" ]]; then
                UPDATE_CLI="$2"
                shift 2
            else
                UPDATE_CLI="true"
                shift
            fi
            ;;
        --update=*)
            # Handle --update=false or --update=true
            UPDATE_CLI="${1#*=}"
            shift
            ;;
        -h | --help)
            echo "GBOX Installation Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -y, --yes, --non-interactive    Run in non-interactive mode (use all defaults)"
            echo "  --with-deps                     Install all command dependencies (adb, frpc, appium)"
            echo "  --update [true|false]           Update GBOX CLI to the latest version (default: true)"
            echo "  --update=false                  Skip update even if already installed"
            echo "  -h, --help                      Show this help message"
            echo ""
            echo "Default behavior:"
            echo "  By default, only GBOX CLI is installed."
            echo "  Use --with-deps to also install all command dependencies."
            echo ""
            echo "Examples:"
            echo "  # Install GBOX CLI only (default)"
            echo "  $0"
            echo ""
            echo "  # Install GBOX CLI with all dependencies"
            echo "  $0 --with-deps"
            echo ""
            echo "  # Update GBOX CLI to latest version"
            echo "  $0 --update"
            echo ""
            echo "  # Non-interactive installation"
            echo "  $0 -y"
            echo ""
            echo "  # Non-interactive with all dependencies"
            echo "  $0 -y --with-deps"
            echo ""
            echo "  # Using curl (CLI only)"
            echo "  curl -fsSL https://raw.githubusercontent.com/babelcloud/gbox/main/install.sh | bash"
            echo ""
            echo "  # Using curl with dependencies"
            echo "  curl -fsSL https://raw.githubusercontent.com/babelcloud/gbox/main/install.sh | bash -s -- --with-deps"
            echo ""
            echo "  # Using curl to update"
            echo "  curl -fsSL https://raw.githubusercontent.com/babelcloud/gbox/main/install.sh | bash -s -- --update"
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
        esac
    done
}

print_header() {
    echo -e "${BLUE}${BOLD}"
    echo "╔════════════════════════════════════════╗"
    echo "║                                        ║"
    echo "║        GBOX Installation Script        ║"
    echo "║                                        ║"
    echo "╚════════════════════════════════════════╝"
    echo -e "${NC}"
}

print_success() {
    echo -e "${GREEN}✅  ${NC}$1"
}

print_warning() {
    echo -e "${YELLOW}⚠️  ${NC}$1"
}

print_error() {
    echo -e "${RED}❌  ${NC}$1"
}

print_info() {
    echo -e "${BLUE}ℹ️  ${NC}$1"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Execute command with sudo if available and needed
run_as_root() {
    # If already root, no need for sudo
    if [ "$(id -u)" = "0" ]; then
        "$@"
    # If sudo is available, use it
    elif command_exists sudo; then
        sudo "$@"
    # Otherwise, try to run directly
    else
        "$@"
    fi
}

# Detect OS and set installation method
detect_os() {
    case "$OS" in
    Linux*)
        OS_TYPE="linux"
        if command_exists apt-get; then
            PKG_MANAGER="apt"
        elif command_exists yum; then
            PKG_MANAGER="yum"
        elif command_exists dnf; then
            PKG_MANAGER="dnf"
        else
            PKG_MANAGER="unknown"
        fi
        ;;
    Darwin*)
        OS_TYPE="macos"
        PKG_MANAGER="brew"
        ;;
    MINGW* | MSYS* | CYGWIN*)
        OS_TYPE="windows"
        PKG_MANAGER="choco"
        ;;
    *)
        OS_TYPE="unknown"
        PKG_MANAGER="unknown"
        ;;
    esac
}

# Check Node.js installation
check_nodejs() {
    if command_exists node && command_exists npm; then
        NODE_VERSION=$(node --version)
        NPM_VERSION=$(npm --version)
        print_success "Node.js $NODE_VERSION and npm $NPM_VERSION are installed"
        return 0
    else
        return 1
    fi
}

# Install Node.js using different methods
install_nodejs_nvm() {
    print_info "Installing Node.js using nvm (Node Version Manager)..."

    # Install nvm
    if ! command_exists nvm; then
        curl -so- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.3/install.sh | bash

        # Load nvm
        export NVM_DIR="$HOME/.nvm"
        [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
    fi

    # Install LTS version
    nvm install --lts
    nvm use --lts

    print_success "Node.js installed via nvm"
}

install_nodejs_brew() {
    print_info "Installing Node.js using Homebrew..."

    if ! command_exists brew; then
        print_error "Homebrew is not installed. Installing Homebrew first..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    fi

    brew install node
    print_success "Node.js installed via Homebrew"
}

install_nodejs_apt() {
    print_info "Installing Node.js using apt..."

    # Install Node.js 20.x LTS
    curl -fsSL https://deb.nodesource.com/setup_20.x | run_as_root bash -
    run_as_root apt-get install -y nodejs

    print_success "Node.js installed via apt"
}

install_nodejs_yum() {
    print_info "Installing Node.js using yum..."

    # Install Node.js 20.x LTS
    curl -fsSL https://rpm.nodesource.com/setup_20.x | run_as_root bash -
    run_as_root yum install -y nodejs

    print_success "Node.js installed via yum"
}

install_nodejs_windows() {
    print_info "Installing Node.js on Windows..."
    print_warning "Please download and install Node.js manually from https://nodejs.org/"
    print_warning "After installation, restart your terminal and re-run this script."
    exit 1
}

# Interactive Node.js installation
prompt_install_nodejs() {
    echo ""
    echo -e "${YELLOW}${BOLD}Node.js is required for Appium automation${NC}"
    echo ""

    # Non-interactive mode: use default option (Homebrew on macOS, package manager on Linux)
    if [ "$NON_INTERACTIVE" = true ]; then
        print_info "Non-interactive mode: Using default installation method"
        case "$OS_TYPE" in
        macos)
            install_nodejs_brew
            ;;
        linux)
            if [ "$PKG_MANAGER" = "apt" ]; then
                install_nodejs_apt
            elif [ "$PKG_MANAGER" = "yum" ] || [ "$PKG_MANAGER" = "dnf" ]; then
                install_nodejs_yum
            else
                print_error "Unsupported package manager: $PKG_MANAGER"
                return 1
            fi
            ;;
        windows)
            install_nodejs_windows
            ;;
        *)
            print_error "Unsupported operating system: $OS_TYPE"
            return 1
            ;;
        esac
        return 0
    fi

    # Interactive mode
    case "$OS_TYPE" in
    macos)
        echo "Choose installation method:"
        echo "  1) Homebrew (recommended)"
        echo "  2) nvm (Node Version Manager)"
        echo "  3) Skip Node.js installation"
        echo ""
        read -p "Enter choice [1-3]: " choice

        case $choice in
        1)
            install_nodejs_brew
            ;;
        2)
            install_nodejs_nvm
            ;;
        3)
            print_warning "Skipping Node.js installation"
            return 1
            ;;
        *)
            print_error "Invalid choice"
            return 1
            ;;
        esac
        ;;
    linux)
        echo "Choose installation method:"
        echo "  1) Package Manager ($PKG_MANAGER)"
        echo "  2) nvm (Node Version Manager)"
        echo "  3) Skip Node.js installation"
        echo ""
        read -p "Enter choice [1-3]: " choice

        case $choice in
        1)
            if [ "$PKG_MANAGER" = "apt" ]; then
                install_nodejs_apt
            elif [ "$PKG_MANAGER" = "yum" ] || [ "$PKG_MANAGER" = "dnf" ]; then
                install_nodejs_yum
            else
                print_error "Unsupported package manager: $PKG_MANAGER"
                return 1
            fi
            ;;
        2)
            install_nodejs_nvm
            ;;
        3)
            print_warning "Skipping Node.js installation"
            return 1
            ;;
        *)
            print_error "Invalid choice"
            return 1
            ;;
        esac
        ;;
    windows)
        install_nodejs_windows
        ;;
    *)
        print_error "Unsupported operating system: $OS_TYPE"
        return 1
        ;;
    esac

    return 0
}

# Check if GBOX is installed and get version info (fast, local check only)
check_gbox_installed() {
    if command_exists gbox; then
        # Use gbox version for fast local check (works on all platforms)
        local installed_version=$(gbox version -o json 2>/dev/null | grep '"Version"' | head -1 | sed 's/.*: "\(.*\)".*/\1/' || echo "unknown")
        if [ -n "$installed_version" ] && [ "$installed_version" != "unknown" ]; then
            echo "installed|$installed_version"
            return 0
        fi
    fi

    echo "not_installed"
    return 0
}

# Install GBOX CLI
install_gbox() {
    print_info "Installing GBOX CLI..."

    case "$OS_TYPE" in
    macos)
        if command_exists brew; then
            brew install gbox
            print_success "GBOX CLI installed via Homebrew"
        else
            print_error "Homebrew is required for GBOX installation on macOS"
            print_info "Install Homebrew: /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
            return 1
        fi
        ;;
    linux | windows)
        npm install -g @gbox.ai/cli
        print_success "GBOX CLI installed via npm"
        ;;
    *)
        print_error "Unsupported operating system: $OS_TYPE"
        return 1
        ;;
    esac
}

# Update GBOX CLI
update_gbox() {
    print_info "Updating GBOX CLI..."

    case "$OS_TYPE" in
    macos)
        if command_exists brew; then
            # Update Homebrew first to get latest package info
            print_info "Updating Homebrew repository..."
            if brew update >/dev/null 2>&1; then
                print_success "Homebrew repository updated"
            else
                print_warning "Homebrew update failed, continuing anyway..."
            fi

            # Check which tap is installed
            print_info "Upgrading GBOX CLI..."
            if brew list --formula | grep -q "babelcloud/gru/gbox"; then
                # Using custom tap
                brew upgrade babelcloud/gru/gbox 2>&1 | tail -5
            else
                # Using official formula
                brew upgrade gbox 2>&1 | tail -5
            fi

            # Get updated version
            local new_version=$(gbox version -o json 2>/dev/null | grep '"Version"' | head -1 | sed 's/.*: "\(.*\)".*/\1/' || echo "unknown")
            if [ "$new_version" != "unknown" ]; then
                print_success "GBOX CLI updated to v$new_version"
            else
                print_success "GBOX CLI updated via Homebrew"
            fi
        else
            print_error "Homebrew is required for GBOX update on macOS"
            return 1
        fi
        ;;
    linux | windows)
        print_info "Updating GBOX CLI via npm package @gbox.ai/cli..."
        npm install -g @gbox.ai/cli@latest

        # Get updated version
        local new_version=$(gbox version -o json 2>/dev/null | grep '"Version"' | head -1 | sed 's/.*: "\(.*\)".*/\1/' || echo "unknown")
        if [ "$new_version" != "unknown" ]; then
            print_success "GBOX CLI updated to v$new_version"
        else
            print_success "GBOX CLI updated via npm"
        fi
        ;;
    *)
        print_error "Unsupported operating system: $OS_TYPE"
        return 1
        ;;
    esac
}

# Install frpc from GitHub releases
install_frpc_from_github() {
    local os_type=""
    local arch_type=""

    # Ensure jq is installed for JSON parsing
    if ! command_exists jq; then
        print_info "Installing jq for JSON parsing..."
        case "$OS_TYPE" in
        macos)
            if command_exists brew; then
                brew install jq >/dev/null 2>&1 || {
                    print_error "Failed to install jq"
                    return 1
                }
            else
                print_error "Homebrew not found, cannot install jq"
                return 1
            fi
            ;;
        linux)
            if command -v apt-get >/dev/null 2>&1; then
                run_as_root apt-get update >/dev/null 2>&1
                run_as_root apt-get install -y jq >/dev/null 2>&1 || {
                    print_error "Failed to install jq via apt-get"
                    return 1
                }
            elif command -v yum >/dev/null 2>&1; then
                run_as_root yum install -y jq >/dev/null 2>&1 || {
                    print_error "Failed to install jq via yum"
                    return 1
                }
            else
                print_error "No package manager found, cannot install jq"
                return 1
            fi
            ;;
        *)
            print_error "Unsupported OS for jq installation"
            return 1
            ;;
        esac

        if ! command_exists jq; then
            print_error "jq installation failed, cannot proceed"
            return 1
        fi
        print_success "jq installed successfully"
    fi

    # Get latest frpc version from GitHub API
    print_info "Fetching latest frpc version from GitHub..."
    local frpc_version
    if command_exists curl; then
        frpc_version=$(curl -fsS https://api.github.com/repos/fatedier/frp/releases/latest | jq -r '.tag_name' | sed 's/^v//')
    elif command_exists wget; then
        frpc_version=$(wget -qO- https://api.github.com/repos/fatedier/frp/releases/latest | jq -r '.tag_name' | sed 's/^v//')
    else
        print_error "Neither curl nor wget found"
        return 1
    fi

    if [ -z "$frpc_version" ] || [ "$frpc_version" = "null" ]; then
        print_error "Failed to fetch frpc version from GitHub"
        return 1
    fi

    # Detect OS
    case "$OS" in
    Linux*)
        os_type="linux"
        ;;
    Darwin*)
        os_type="darwin"
        ;;
    *)
        print_error "Unsupported OS for frpc installation: $OS"
        return 1
        ;;
    esac

    # Detect architecture
    case "$ARCH" in
    x86_64 | amd64)
        arch_type="amd64"
        ;;
    aarch64 | arm64)
        arch_type="arm64"
        ;;
    armv7l | armhf)
        arch_type="arm"
        ;;
    *)
        print_error "Unsupported architecture for frpc installation: $ARCH"
        return 1
        ;;
    esac

    local download_url="https://github.com/fatedier/frp/releases/download/v${frpc_version}/frp_${frpc_version}_${os_type}_${arch_type}.tar.gz"
    local temp_dir=$(mktemp -d)
    local tar_file="${temp_dir}/frp.tar.gz"

    print_info "Downloading frpc v${frpc_version} for ${os_type}_${arch_type}..."

    # Download
    if command_exists curl; then
        if ! curl -fsSL -o "$tar_file" "$download_url" 2>&1; then
            print_error "Failed to download frpc"
            rm -rf "$temp_dir"
            return 1
        fi
    elif command_exists wget; then
        if ! wget -O "$tar_file" "$download_url" 2>&1; then
            print_error "Failed to download frpc"
            rm -rf "$temp_dir"
            return 1
        fi
    else
        print_error "Neither curl nor wget found. Please install one of them first."
        rm -rf "$temp_dir"
        return 1
    fi

    # Extract
    print_info "Extracting frpc..."
    if ! tar -xzf "$tar_file" -C "$temp_dir" 2>&1; then
        print_error "Failed to extract frpc"
        rm -rf "$temp_dir"
        return 1
    fi

    # Find and install frpc binary
    local frpc_binary=$(find "$temp_dir" -name "frpc" -type f | head -1)
    if [ -z "$frpc_binary" ]; then
        print_error "frpc binary not found in extracted files"
        rm -rf "$temp_dir"
        return 1
    fi

    # Install to /usr/local/bin
    print_info "Installing frpc to /usr/local/bin..."
    if run_as_root install -m 755 "$frpc_binary" /usr/local/bin/frpc 2>&1; then
        print_success "frpc installed successfully"
    else
        print_error "Failed to install frpc to /usr/local/bin"
        rm -rf "$temp_dir"
        return 1
    fi

    # Cleanup
    rm -rf "$temp_dir"

    return 0
}

# Install additional dependencies
install_dependencies() {
    echo ""
    echo -e "${BLUE}${BOLD}🔧 Required Dependencies${NC}"
    echo "──────────────────────────────────────────"
    echo ""

    # Check and install ADB (required, no prompt)
    if command_exists adb; then
        ADB_VERSION=$(adb version 2>/dev/null | head -1 || echo "unknown")
        print_success "ADB: $ADB_VERSION"
    else
        print_info "Installing Android Debug Bridge (ADB)..."
        case "$OS_TYPE" in
        macos)
            if brew install android-platform-tools; then
                print_success "ADB installed successfully"
            else
                echo ""
                print_error "Failed to install ADB"
                echo ""
                echo "Please install ADB manually:"
                echo "  brew install android-platform-tools"
                echo ""
                exit 1
            fi
            ;;
        linux)
            if [ "$PKG_MANAGER" = "apt" ]; then
                if run_as_root apt-get install -y android-tools-adb; then
                    print_success "ADB installed successfully"
                else
                    echo ""
                    print_error "Failed to install ADB"
                    echo ""
                    echo "Please install ADB manually:"
                    echo "  apt-get install android-tools-adb"
                    echo ""
                    exit 1
                fi
            else
                echo ""
                print_error "Unsupported package manager for ADB installation"
                echo ""
                echo "Please install ADB manually for your system"
                echo ""
                exit 1
            fi
            ;;
        *)
            echo ""
            print_error "Unsupported OS for automatic ADB installation"
            echo ""
            echo "Please install ADB manually from:"
            echo "  https://developer.android.com/tools/releases/platform-tools"
            echo ""
            exit 1
            ;;
        esac
    fi

    # Check and install frpc
    if command_exists frpc; then
        FRPC_VERSION=$(frpc -v 2>/dev/null | head -1 || echo "installed")
        print_success "frpc: $FRPC_VERSION"
    else
        print_info "Installing FRP Client (frpc)..."

        # Try Homebrew first on macOS
        if [ "$OS_TYPE" = "macos" ] && command_exists brew; then
            if brew install frpc 2>&1; then
                print_success "frpc installed successfully via Homebrew"
            else
                print_warning "Homebrew installation failed, trying GitHub releases..."
                if ! install_frpc_from_github; then
                    echo ""
                    print_error "Failed to install frpc"
                    echo ""
                    echo "Please install frpc manually from:"
                    echo "  https://github.com/fatedier/frp/releases"
                    echo ""
                    exit 1
                fi
            fi
        else
            # For Linux and other systems, download from GitHub
            if ! install_frpc_from_github; then
                echo ""
                print_error "Failed to install frpc"
                echo ""
                echo "Please install frpc manually from:"
                echo "  https://github.com/fatedier/frp/releases"
                echo ""
                exit 1
            fi
        fi
    fi

    echo ""
}

# Detect and setup JSON parser (jq)
setup_json_parser() {
    # Check if jq is already installed
    if command -v jq >/dev/null 2>&1; then
        JSON_PARSER="jq"
        return 0
    fi

    # Try to install jq
    print_info "jq not found, installing..."
    case "$OS_TYPE" in
    macos)
        if brew install jq 2>&1; then
            JSON_PARSER="jq"
            print_success "jq installed successfully"
            return 0
        fi
        ;;
    linux)
        if command -v apt-get >/dev/null 2>&1; then
            if run_as_root apt-get install -y jq; then
                JSON_PARSER="jq"
                print_success "jq installed successfully"
                return 0
            fi
        elif command -v yum >/dev/null 2>&1; then
            if run_as_root yum install -y jq; then
                JSON_PARSER="jq"
                print_success "jq installed successfully"
                return 0
            fi
        fi
        ;;
    esac

    # If jq installation failed, use grep fallback
    print_warning "jq installation failed, will use grep fallback"
    JSON_PARSER="grep"
    return 0
}

# Extract version from JSON using jq or grep fallback
extract_json_version() {
    local json_data="$1"
    local key="$2"

    if [ "$JSON_PARSER" = "jq" ]; then
        # Use jq for accurate JSON parsing
        echo "$json_data" | jq -r ".${key}.version // empty" 2>/dev/null
    else
        # Fallback to grep method
        echo "$json_data" | grep -A 20 "\"$key\"" | grep '"version"' | head -1 | sed 's/.*: "\(.*\)".*/\1/'
    fi
}

# Spinner animation for background processes
spinner() {
    local pid=$1
    local message=$2
    local spinstr='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
    local i=0

    while kill -0 $pid 2>/dev/null; do
        local temp=${spinstr:i++%${#spinstr}:1}
        printf "\r  ${BLUE}%s${NC} %s..." "$temp" "$message"
        sleep 0.1
    done
    printf "\r%80s\r" ""
}

# Install Appium and components with beautiful formatting
install_appium_components() {
    echo ""
    echo -e "${BLUE}${BOLD}🚀 Installing Appium Automation${NC}"
    echo "──────────────────────────────────────────"
    echo ""

    APPIUM_PATH="$HOME/.gbox/device-proxy/appium"
    APPIUM_BIN="$APPIUM_PATH/node_modules/.bin/appium"

    # Use default configuration
    DRIVERS="${GBOX_APPIUM_DRIVERS:-uiautomator2}"
    PLUGINS="${GBOX_APPIUM_PLUGINS:-inspector}"

    # Create directory
    mkdir -p "$APPIUM_PATH"

    # Install Appium Server
    if [ -f "$APPIUM_BIN" ]; then
        APPIUM_VERSION=$("$APPIUM_BIN" --version 2>/dev/null || echo "unknown")
        echo -e "  ${GREEN}✓${NC} Appium Server ${DIM}v${APPIUM_VERSION}${NC}"
    else
        npm install appium --prefix "$APPIUM_PATH" --silent &
        spinner $! "Installing Appium Server"
        wait $!
        if [ $? -eq 0 ]; then
            APPIUM_VERSION=$("$APPIUM_BIN" --version 2>/dev/null || echo "unknown")
            echo -e "  ${GREEN}✓${NC} Appium Server ${DIM}v${APPIUM_VERSION}${NC}"
        else
            echo -e "  ${RED}✗${NC} Appium Server ${DIM}(failed)${NC}"
            return 1
        fi
    fi

    # Install Drivers
    if [ -n "$DRIVERS" ]; then
        echo ""
        echo -e "${DIM}  Drivers:${NC}"
        IFS=',' read -ra DRIVER_ARRAY <<<"$DRIVERS"
        for driver in "${DRIVER_ARRAY[@]}"; do
            driver=$(echo "$driver" | xargs)
            if [ -z "$driver" ]; then continue; fi

            # Check if already installed
            DRIVER_INFO=$(APPIUM_HOME="$APPIUM_PATH" "$APPIUM_BIN" driver list --installed --json 2>/dev/null || echo "{}")
            if echo "$DRIVER_INFO" | grep -q "\"$driver\""; then
                DRIVER_VERSION=$(extract_json_version "$DRIVER_INFO" "$driver")
                echo -e "    ${GREEN}✓${NC} ${driver} ${DIM}v${DRIVER_VERSION}${NC}"
            else
                APPIUM_HOME="$APPIUM_PATH" "$APPIUM_BIN" driver install "$driver" >/dev/null 2>&1 &
                spinner $! "Installing ${driver}"
                wait $!
                if [ $? -eq 0 ]; then
                    DRIVER_INFO=$(APPIUM_HOME="$APPIUM_PATH" "$APPIUM_BIN" driver list --installed --json 2>/dev/null || echo "{}")
                    DRIVER_VERSION=$(extract_json_version "$DRIVER_INFO" "$driver")
                    echo -e "    ${GREEN}✓${NC} ${driver} ${DIM}v${DRIVER_VERSION}${NC}"
                else
                    echo -e "    ${RED}✗${NC} ${driver} ${DIM}(failed)${NC}"
                fi
            fi
        done
    fi

    # Install Plugins
    if [ -n "$PLUGINS" ]; then
        echo ""
        echo -e "${DIM}  Plugins:${NC}"
        IFS=',' read -ra PLUGIN_ARRAY <<<"$PLUGINS"
        for plugin in "${PLUGIN_ARRAY[@]}"; do
            plugin=$(echo "$plugin" | xargs)
            if [ -z "$plugin" ]; then continue; fi

            # Check if already installed
            PLUGIN_INFO=$(APPIUM_HOME="$APPIUM_PATH" "$APPIUM_BIN" plugin list --installed --json 2>/dev/null || echo "{}")
            if echo "$PLUGIN_INFO" | grep -q "\"$plugin\""; then
                PLUGIN_VERSION=$(extract_json_version "$PLUGIN_INFO" "$plugin")
                echo -e "    ${GREEN}✓${NC} ${plugin} ${DIM}v${PLUGIN_VERSION}${NC}"
            else
                APPIUM_HOME="$APPIUM_PATH" "$APPIUM_BIN" plugin install "$plugin" >/dev/null 2>&1 &
                spinner $! "Installing ${plugin}"
                wait $!
                if [ $? -eq 0 ]; then
                    PLUGIN_INFO=$(APPIUM_HOME="$APPIUM_PATH" "$APPIUM_BIN" plugin list --installed --json 2>/dev/null || echo "{}")
                    PLUGIN_VERSION=$(extract_json_version "$PLUGIN_INFO" "$plugin")
                    echo -e "    ${GREEN}✓${NC} ${plugin} ${DIM}v${PLUGIN_VERSION}${NC}"
                else
                    echo -e "    ${RED}✗${NC} ${plugin} ${DIM}(failed)${NC}"
                fi
            fi
        done
    fi

    echo ""
    echo "──────────────────────────────────────────"
    echo -e "  ${GREEN}✓${NC} Installation completed"
    echo -e "  ${DIM}Location: ${APPIUM_PATH}${NC}"
    echo ""
}

# Check Appium installation status (populates global MISSING_COMPONENTS array)
check_appium_status() {
    APPIUM_PATH="$HOME/.gbox/device-proxy/appium"
    APPIUM_BIN="$APPIUM_PATH/node_modules/.bin/appium"

    REQUIRED_DRIVERS="${GBOX_APPIUM_DRIVERS:-uiautomator2}"
    REQUIRED_PLUGINS="${GBOX_APPIUM_PLUGINS:-inspector}"

    # Use global variable so it's accessible outside the function
    MISSING_COMPONENTS=()

    # Check Appium Server
    if [ ! -f "$APPIUM_BIN" ]; then
        MISSING_COMPONENTS+=("Appium Server")
    fi

    # Check Drivers (only if Appium is installed)
    if [ -f "$APPIUM_BIN" ]; then
        IFS=',' read -ra DRIVER_ARRAY <<<"$REQUIRED_DRIVERS"
        for driver in "${DRIVER_ARRAY[@]}"; do
            driver=$(echo "$driver" | xargs)
            if [ -z "$driver" ]; then continue; fi

            DRIVER_INFO=$(APPIUM_HOME="$APPIUM_PATH" "$APPIUM_BIN" driver list --installed --json 2>/dev/null || echo "{}")
            if ! echo "$DRIVER_INFO" | grep -q "\"$driver\""; then
                MISSING_COMPONENTS+=("Driver: $driver")
            fi
        done

        # Check Plugins
        IFS=',' read -ra PLUGIN_ARRAY <<<"$REQUIRED_PLUGINS"
        for plugin in "${PLUGIN_ARRAY[@]}"; do
            plugin=$(echo "$plugin" | xargs)
            if [ -z "$plugin" ]; then continue; fi

            PLUGIN_INFO=$(APPIUM_HOME="$APPIUM_PATH" "$APPIUM_BIN" plugin list --installed --json 2>/dev/null || echo "{}")
            if ! echo "$PLUGIN_INFO" | grep -q "\"$plugin\""; then
                MISSING_COMPONENTS+=("Plugin: $plugin")
            fi
        done
    fi

    # Return status
    if [ ${#MISSING_COMPONENTS[@]} -eq 0 ]; then
        return 0 # All components installed
    else
        return 1 # Missing components
    fi
}

# Display Appium installation status
display_appium_status() {
    APPIUM_PATH="$HOME/.gbox/device-proxy/appium"
    APPIUM_BIN="$APPIUM_PATH/node_modules/.bin/appium"

    echo ""
    echo -e "${BLUE}${BOLD}🚀 Appium Status${NC}"
    echo "──────────────────────────────────────────"
    echo ""

    if [ -f "$APPIUM_BIN" ]; then
        APPIUM_VERSION=$("$APPIUM_BIN" --version 2>/dev/null || echo "unknown")
        echo -e "  ${GREEN}✓${NC} Appium Server ${DIM}v${APPIUM_VERSION}${NC}"

        # Show drivers
        DRIVERS="${GBOX_APPIUM_DRIVERS:-uiautomator2}"
        IFS=',' read -ra DRIVER_ARRAY <<<"$DRIVERS"
        if [ ${#DRIVER_ARRAY[@]} -gt 0 ]; then
            echo ""
            echo -e "${DIM}  Drivers:${NC}"
            for driver in "${DRIVER_ARRAY[@]}"; do
                driver=$(echo "$driver" | xargs)
                if [ -z "$driver" ]; then continue; fi

                DRIVER_INFO=$(APPIUM_HOME="$APPIUM_PATH" "$APPIUM_BIN" driver list --installed --json 2>/dev/null || echo "{}")
                if echo "$DRIVER_INFO" | grep -q "\"$driver\""; then
                    DRIVER_VERSION=$(extract_json_version "$DRIVER_INFO" "$driver")
                    if [ -n "$DRIVER_VERSION" ]; then
                        echo -e "    ${GREEN}✓${NC} ${driver} ${DIM}v${DRIVER_VERSION}${NC}"
                    else
                        echo -e "    ${RED}✗${NC} ${driver} ${DIM}(failed)${NC}"
                    fi
                fi
            done
        fi

        # Show plugins
        PLUGINS="${GBOX_APPIUM_PLUGINS:-inspector}"
        IFS=',' read -ra PLUGIN_ARRAY <<<"$PLUGINS"
        if [ ${#PLUGIN_ARRAY[@]} -gt 0 ]; then
            echo ""
            echo -e "${DIM}  Plugins:${NC}"
            for plugin in "${PLUGIN_ARRAY[@]}"; do
                plugin=$(echo "$plugin" | xargs)
                if [ -z "$plugin" ]; then continue; fi

                PLUGIN_INFO=$(APPIUM_HOME="$APPIUM_PATH" "$APPIUM_BIN" plugin list --installed --json 2>/dev/null || echo "{}")
                if echo "$PLUGIN_INFO" | grep -q "\"$plugin\""; then
                    PLUGIN_VERSION=$(extract_json_version "$PLUGIN_INFO" "$plugin")
                    if [ -n "$PLUGIN_VERSION" ]; then
                        echo -e "    ${GREEN}✓${NC} ${plugin} ${DIM}v${PLUGIN_VERSION}${NC}"
                    else
                        echo -e "    ${RED}✗${NC} ${plugin} ${DIM}(failed)${NC}"
                    fi
                fi
            done
        fi

        echo ""
        echo "──────────────────────────────────────────"
        echo -e "  ${DIM}Location: ${APPIUM_PATH}${NC}"
    fi
    echo ""
}

# Configure Appium installation
configure_appium() {
    echo ""
    echo -e "${BLUE}${BOLD}🚀 Appium Automation Setup${NC}"
    echo "───────────────────────────────────────────"

    # Check if Appium is disabled via environment variable
    if [ "${GBOX_APPIUM_DISABLED}" = "true" ]; then
        echo ""
        print_info "Appium installation is disabled (GBOX_APPIUM_DISABLED=true)"
        print_warning "Device automation features will not be available"
        echo ""
        return 0
    fi

    # Set default configuration
    export GBOX_APPIUM_DRIVERS="${GBOX_APPIUM_DRIVERS:-uiautomator2}"
    export GBOX_APPIUM_PLUGINS="${GBOX_APPIUM_PLUGINS:-inspector}"

    # Export APPIUM_HOME for user convenience
    export APPIUM_HOME="$HOME/.gbox/device-proxy/appium"

    echo ""
    print_info "Required components:"
    echo "  📦 Appium Server"
    echo "  🔧 Driver:  ${GBOX_APPIUM_DRIVERS}"
    echo "  🔌 Plugin:  ${GBOX_APPIUM_PLUGINS}"

    # Check current installation status
    echo ""
    if check_appium_status; then
        print_success "All Appium components are already installed!"
        display_appium_status
        export GBOX_INSTALL_APPIUM=true
        return 0
    fi

    # List missing components
    echo ""
    echo -e "${YELLOW}Missing components detected:${NC}"
    for component in "${MISSING_COMPONENTS[@]}"; do
        echo "  ⚠️  $component"
    done

    # Automatically install missing components
    echo ""
    print_info "Installing missing components automatically..."
    export GBOX_INSTALL_APPIUM=true

    if install_appium_components; then
        echo ""
        print_success "Appium installation completed successfully!"
    else
        echo ""
        echo -e "${RED}${BOLD}╔════════════════════════════════════════════════════╗${NC}"
        echo -e "${RED}${BOLD}║  ⚠️  WARNING: Installation Failed                 ║${NC}"
        echo -e "${RED}${BOLD}╚════════════════════════════════════════════════════╝${NC}"
        echo ""
        echo -e "${YELLOW}Some components failed to install. You may experience:${NC}"
        echo -e "  ${RED}❌${NC} Device connection issues"
        echo -e "  ${RED}❌${NC} Automation test failures"
        echo ""
        echo -e "${YELLOW}To skip Appium installation, set:${NC}"
        echo -e "  ${BLUE}export GBOX_APPIUM_DISABLED=true${NC}"
        echo ""
        echo -e "${YELLOW}To retry installation, run:${NC}"
        echo -e "  ${BLUE}curl -fsSL https://raw.githubusercontent.com/babelcloud/gbox/main/install.sh | bash${NC}"
        echo ""
        return 1
    fi
}

# Save configuration to profile
save_config() {
    SHELL_RC=""
    if [ -n "$BASH_VERSION" ]; then
        SHELL_RC="$HOME/.bashrc"
    elif [ -n "$ZSH_VERSION" ]; then
        SHELL_RC="$HOME/.zshrc"
    fi

    if [ -n "$SHELL_RC" ] && [ -f "$SHELL_RC" ]; then
        # Check if config already exists
        if ! grep -q "GBOX Appium Configuration" "$SHELL_RC"; then
            echo "" >>"$SHELL_RC"
            echo "# GBOX Appium Configuration" >>"$SHELL_RC"
            echo "export GBOX_INSTALL_APPIUM=${GBOX_INSTALL_APPIUM:-true}" >>"$SHELL_RC"
            echo "export GBOX_APPIUM_DRIVERS=${GBOX_APPIUM_DRIVERS:-uiautomator2}" >>"$SHELL_RC"
            echo "export GBOX_APPIUM_PLUGINS=${GBOX_APPIUM_PLUGINS:-inspector}" >>"$SHELL_RC"
            echo "export APPIUM_HOME=\"\$HOME/.gbox/device-proxy/appium\"" >>"$SHELL_RC"

            print_success "Configuration saved to $SHELL_RC"
        else
            print_info "Configuration already exists in $SHELL_RC"
        fi
    fi
}

# Print next steps
print_next_steps() {
    echo ""
    echo -e "${GREEN}${BOLD}╔════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}${BOLD}║                                        ║${NC}"
    echo -e "${GREEN}${BOLD}║       ✅ Installation Complete!        ║${NC}"
    echo -e "${GREEN}${BOLD}║                                        ║${NC}"
    echo -e "${GREEN}${BOLD}╚════════════════════════════════════════╝${NC}"
    echo ""
    echo "──────────────────────────────────────────"
    echo ""
    echo -e "${BOLD}📋 Next Steps:${NC}"
    echo ""
    echo "1️⃣  Reload your shell configuration:"
    echo -e "   ${YELLOW}source ~/.bashrc${NC} (or ~/.zshrc)"
    echo ""
    echo "2️⃣  Login to GBOX:"
    echo -e "   ${YELLOW}gbox login${NC}"
    echo ""
    echo "3️⃣  Connect your device:"
    echo -e "   ${YELLOW}gbox device-connect${NC}"
    echo ""
    echo "4️⃣  Export MCP config (optional):"
    echo -e "   ${YELLOW}gbox mcp export --merge-to cursor${NC}"
    echo ""
    echo -e "${BLUE}ℹ️  For more information, visit: https://docs.gbox.ai${NC}"
    echo ""
}

# Main installation flow
main() {
    print_header

    if [ "$NON_INTERACTIVE" = true ]; then
        print_info "Running in non-interactive mode (all defaults will be used)"
        echo ""
    fi

    if [ "$WITH_DEPS" = true ]; then
        print_info "Installing GBOX CLI with all dependencies"
    else
        print_info "Installing GBOX CLI only (use --with-deps for dependencies)"
    fi
    echo ""

    # Detect OS first
    detect_os

    print_info "Detected OS: $OS_TYPE ($OS $ARCH)\n"
    print_info "Package Manager: $PKG_MANAGER"
    echo ""
    echo "───────────────────────────────────────"
    echo ""

    # Only check Node.js if --with-deps is specified (required for Appium)
    if [ "$WITH_DEPS" = true ]; then
        # Setup JSON parser for Appium version extraction
        setup_json_parser

        if ! check_nodejs; then
            echo ""
            print_warning "Node.js is not installed"
            echo ""
            if ! prompt_install_nodejs; then
                echo ""
                echo -e "${RED}${BOLD}╔═════════════════════════════════════════════════════╗${NC}"
                echo -e "${RED}${BOLD}║  ❌  Installation Cancelled                       ║${NC}"
                echo -e "${RED}${BOLD}╚════════════════════════════════════════════════════╝${NC}"
                echo ""
                echo -e "${YELLOW}Node.js is required for Appium automation features.${NC}"
                echo ""
                echo "Please install Node.js first:"
                echo "  🍎 macOS:         brew install node"
                echo "  🐧 Ubuntu/Debian: sudo apt-get install nodejs npm"
                echo "  🪟 Windows:       Download from https://nodejs.org/"
                echo ""
                echo "Or use our installation script with Node.js pre-installed."
                echo ""
                exit 1
            fi
        fi
    fi

    # Install GBOX CLI
    echo ""
    GBOX_STATUS=$(check_gbox_installed)

    if [[ "$GBOX_STATUS" == "not_installed" ]]; then
        # GBOX not installed, install it
        install_gbox
    else
        # GBOX is already installed
        IFS='|' read -ra STATUS_PARTS <<<"$GBOX_STATUS"
        INSTALLED_VERSION="${STATUS_PARTS[1]}"

        print_success "GBOX CLI is already installed: v$INSTALLED_VERSION"
        echo ""

        # Handle update based on UPDATE_CLI flag
        if [ "$UPDATE_CLI" = "true" ]; then
            # Explicitly requested update
            print_info "Updating GBOX CLI to the latest version..."
            update_gbox
        elif [ "$UPDATE_CLI" = "false" ]; then
            # Explicitly disabled update
            print_info "Update skipped (--update=false)"
        elif [ "$UPDATE_CLI" = "" ]; then
            # No --update flag specified
            if [ "$NON_INTERACTIVE" = true ]; then
                print_info "Non-interactive mode: Skipping update"
            else
                # Interactive mode: ask if user wants to update
                read -p "Update GBOX CLI to the latest version? [y/N]: " update_choice
                if [[ "$update_choice" =~ ^[Yy]$ ]]; then
                    update_gbox
                fi
            fi
        fi
    fi

    # Only install dependencies if --with-deps is specified
    if [ "$WITH_DEPS" = true ]; then
        # Install additional dependencies (ADB, frpc) before Appium
        install_dependencies

        # Configure and install Appium automation components
        if [ "$GBOX_INSTALL_APPIUM" != "false" ] && [ "${GBOX_APPIUM_DISABLED}" != "true" ]; then
            configure_appium
        elif [ "${GBOX_APPIUM_DISABLED}" = "true" ]; then
            echo ""
            print_info "Appium installation is disabled (GBOX_APPIUM_DISABLED=true)"
        fi

        # Save configuration
        save_config
    else
        echo ""
        print_info "GBOX CLI installation completed. Use 'gbox setup' to install command dependencies."
    fi

    # Print next steps
    print_next_steps
}

# Parse arguments and run main
parse_args "$@"
main
