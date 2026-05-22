#!/bin/bash
#
# kvelmo Install Script
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash -s -- --nightly
#   curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash -s -- -v v1.2.3
#

set -euo pipefail

REPO="valksor/kvelmo"
BINARY_NAME="kvelmo"
VERSION=""
NIGHTLY=false

# Minisign public key for binary verification
# Generated: 2026-05-22
# Key ID: 7527FDFF95C1D7BB
MINISIGN_PUBLIC_KEY="RWS718GV//0ndQGx85bxSWMv+70MdX0OUELW1QJ3j6KC44Zj2R8pRbTx"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

success() {
    echo -e "${GREEN}[OK]${NC} $1" >&2
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1" >&2
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -n|--nightly)
            NIGHTLY=true
            shift
            ;;
        -h|--help)
            echo "Usage: install.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -v, --version VERSION  Install specific version (e.g., v1.2.3)"
            echo "  -n, --nightly          Install latest nightly build"
            echo "  -h, --help             Show this help message"
            exit 0
            ;;
        *)
            error "Unknown option: $1. Use --help for usage."
            ;;
    esac
done

# Check dependencies
check_dependencies() {
    if ! command -v curl &> /dev/null; then
        error "curl is required but not installed. Please install curl and try again."
    fi
}

# Kill running kvelmo processes to prevent "Text file busy"
kill_running_processes() {
    if pgrep -x "$BINARY_NAME" >/dev/null 2>&1; then
        info "Stopping running $BINARY_NAME processes..."
        pkill -x "$BINARY_NAME" 2>/dev/null || true
        sleep 0.5
    fi
}

# Check if running inside WSL
is_wsl() {
    # Fast path: check WSL_DISTRO_NAME env var
    if [[ -n "${WSL_DISTRO_NAME:-}" ]]; then
        return 0
    fi
    # Fallback: check /proc/version for "microsoft" or "WSL"
    if [[ -f /proc/version ]] && grep -qiE "(microsoft|wsl)" /proc/version 2>/dev/null; then
        return 0
    fi
    return 1
}

# Detect OS
detect_os() {
    local os
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        linux)
            # Note WSL detection for user clarity
            if is_wsl; then
                info "WSL environment detected"
            fi
            echo "linux"
            ;;
        darwin)
            echo "darwin"
            ;;
        mingw*|msys*|cygwin*)
            # Windows compatibility layer - not supported, suggest WSL
            echo ""
            error "Detected $os environment (Windows compatibility layer).

kvelmo requires WSL2 (Windows Subsystem for Linux) on Windows.

To install via WSL:
  1. Install WSL2: wsl --install (in PowerShell as Admin)
  2. Restart your computer
  3. Run this script inside WSL: wsl -e bash -c \"curl -fsSL https://raw.githubusercontent.com/valksor/kvelmo/master/install.sh | bash\"

Or use the PowerShell installer:
  irm https://raw.githubusercontent.com/valksor/kvelmo/master/install.ps1 | iex"
            ;;
        *)
            error "Unsupported operating system: $os. Supported: linux, darwin (macOS)"
            ;;
    esac
}

# Detect architecture
detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)
            echo "amd64"
            ;;
        arm64|aarch64)
            echo "arm64"
            ;;
        *)
            error "Unsupported architecture: $arch. Supported: amd64, arm64"
            ;;
    esac
}

# Get latest stable version from GitHub API
get_latest_version() {
    local version
    version=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [[ -z "$version" ]]; then
        error "Failed to fetch latest version from GitHub API"
    fi
    echo "$version"
}

# Find suitable install directory
find_install_dir() {
    local dirs=("$HOME/.local/bin" "$HOME/bin" "/usr/local/bin")

    for dir in "${dirs[@]}"; do
        # Check if directory exists and is writable
        if [[ -d "$dir" && -w "$dir" ]]; then
            echo "$dir"
            return 0
        fi

        # Try to create user directories
        if [[ "$dir" == "$HOME/.local/bin" || "$dir" == "$HOME/bin" ]]; then
            if mkdir -p "$dir" 2>/dev/null; then
                echo "$dir"
                return 0
            fi
        fi
    done

    # Fall back to /usr/local/bin with sudo
    echo "/usr/local/bin"
}

# Check if directory is in PATH
check_path() {
    local dir="$1"
    if [[ ":$PATH:" != *":$dir:"* ]]; then
        warn "$dir is not in your PATH"
        echo ""
        echo "Add it to your PATH by adding this line to your shell config:"

        local shell_name
        shell_name=$(basename "$SHELL")
        case "$shell_name" in
            bash)
                echo "  echo 'export PATH=\"$dir:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
                ;;
            zsh)
                echo "  echo 'export PATH=\"$dir:\$PATH\"' >> ~/.zshrc && source ~/.zshrc"
                ;;
            *)
                echo "  export PATH=\"$dir:\$PATH\""
                ;;
        esac
        echo ""
    fi
}

# Verify SHA256 checksum
verify_checksum() {
    local file="$1"
    local expected="$2"
    local actual

    if command -v sha256sum &> /dev/null; then
        actual=$(sha256sum "$file" | awk '{print $1}')
    elif command -v shasum &> /dev/null; then
        actual=$(shasum -a 256 "$file" | awk '{print $1}')
    else
        warn "Neither sha256sum nor shasum available - skipping checksum verification"
        return 0
    fi

    if [[ "$actual" != "$expected" ]]; then
        error "Checksum verification failed!\n  Expected: $expected\n  Actual:   $actual"
    fi

    success "Checksum verified"
}

# Verify Minisign signature (if minisign is available)
verify_minisign() {
    local base_url="$1"
    local tmpdir="$2"

    if ! command -v minisign &> /dev/null; then
        info "minisign not found - skipping signature verification"
        info "Install minisign to verify binary authenticity: https://github.com/jedisct1/minisign"
        return 0
    fi

    info "Verifying Minisign signature..."

    # Download checksum signature and checksum file
    local sig_url="${base_url}/checksums.txt.minisig"
    local sig_file="${tmpdir}/checksums.txt.minisig"
    local checksum_url="${base_url}/checksums.txt"
    local checksum_file="${tmpdir}/checksums.txt"

    if ! curl -fsSL "$sig_url" -o "$sig_file" 2>/dev/null; then
        warn "Failed to download signature file - skipping signature verification"
        return 0
    fi

    if ! curl -fsSL "$checksum_url" -o "$checksum_file" 2>/dev/null; then
        warn "Failed to download checksum file - skipping signature verification"
        return 0
    fi

    # Verify with minisign
    if minisign -Vm "$checksum_file" -P "$MINISIGN_PUBLIC_KEY" -x "$sig_file" &>/dev/null; then
        success "Minisign signature verified"
    else
        error "Minisign signature verification failed!"
    fi
}

# Main installation function
main() {
    echo ""
    echo "  kvelmo"
    echo "  Task lifecycle orchestration for AI-assisted development"
    echo ""

    check_dependencies

    local os arch
    os=$(detect_os)
    arch=$(detect_arch)

    info "Detected platform: ${os}/${arch}"

    # Determine version
    if [[ "$NIGHTLY" == true ]]; then
        VERSION="nightly"
        info "Installing nightly build"
    elif [[ -z "$VERSION" ]]; then
        info "Fetching latest stable version..."
        VERSION=$(get_latest_version)
    fi

    info "Version: ${VERSION}"

    # Construct download URLs
    local binary_name="${BINARY_NAME}-${os}-${arch}"
    local base_url="https://github.com/${REPO}/releases/download/${VERSION}"
    local binary_url="${base_url}/${binary_name}"
    local checksum_url="${base_url}/${binary_name}.sha256"

    # Create temporary directory
    local tmpdir
    tmpdir=$(mktemp -d)
    # shellcheck disable=SC2064
    trap "rm -rf '$tmpdir'" EXIT

    local binary_file="${tmpdir}/${BINARY_NAME}"
    local checksum_file="${tmpdir}/checksum.sha256"

    # Download binary
    info "Downloading ${binary_name}..."
    if ! curl -fsSL "$binary_url" -o "$binary_file"; then
        error "Failed to download binary from ${binary_url}"
    fi

    # Download and verify checksum
    info "Downloading checksum..."
    if curl -fsSL "$checksum_url" -o "$checksum_file" 2>/dev/null; then
        local expected_checksum
        expected_checksum=$(cat "$checksum_file" | awk '{print $1}')
        verify_checksum "$binary_file" "$expected_checksum"
    else
        warn "Checksum file not available - skipping checksum verification"
    fi

    # Verify Minisign signature (opportunistic)
    verify_minisign "$base_url" "$tmpdir"

    # Make executable
    chmod +x "$binary_file"

    # Find install directory
    local install_dir
    install_dir=$(find_install_dir)
    local install_path="${install_dir}/${BINARY_NAME}"

    info "Installing to ${install_path}..."

    # Kill any running processes to prevent "Text file busy"
    kill_running_processes

    # Install binary
    if [[ -w "$install_dir" ]]; then
        mv "$binary_file" "$install_path"
    else
        info "Requesting sudo access to install to ${install_dir}..."
        sudo mv "$binary_file" "$install_path"
    fi

    success "Installed ${BINARY_NAME} ${VERSION} to ${install_path}"

    # Check PATH
    check_path "$install_dir"

    # Verify installation
    if command -v "$BINARY_NAME" &> /dev/null; then
        echo ""
        info "Verifying installation..."
        "$BINARY_NAME" version
        echo ""
        success "Installation complete! Run '${BINARY_NAME} --help' to get started."
    else
        echo ""
        success "Installation complete!"
        echo "Run '${install_path} --help' to get started."
        echo "(You may need to restart your shell or update your PATH)"
    fi
}

main "$@"
