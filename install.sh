#!/usr/bin/env bash
# Admirarr installer — works on Linux, macOS, WSL
# Usage: curl -fsSL https://raw.githubusercontent.com/maxtechera/admirarr/main/install.sh | bash
set -e

REPO="maxtechera/admirarr"
BIN_NAME="admirarr"
INSTALL_DIR="${ADMIRARR_INSTALL_DIR:-/usr/local/bin}"

# Colors
GOLD='\033[33m'
GREEN='\033[92m'
RED='\033[91m'
DIM='\033[2m'
BOLD='\033[1m'
RESET='\033[0m'

info()  { printf "${GOLD}⚓${RESET} %s\n" "$1"; }
ok()    { printf "${GREEN}✓${RESET} %s\n" "$1"; }
fail()  { printf "${RED}✗${RESET} %s\n" "$1"; exit 1; }

echo ""
printf "  ${GOLD}⚓ ${BOLD}ADMIRARR${RESET} ${DIM}installer${RESET}\n"
printf "  ${DIM}Command your fleet.${RESET}\n\n"

# Check Python 3
if command -v python3 &>/dev/null; then
    PY_VERSION=$(python3 --version 2>&1 | awk '{print $2}')
    ok "Python ${PY_VERSION} found"
else
    fail "Python 3 is required but not found. Install it first:
    macOS:   brew install python3
    Ubuntu:  sudo apt install python3
    Fedora:  sudo dnf install python3"
fi

# Check Python version >= 3.7
PY_MAJOR=$(python3 -c "import sys; print(sys.version_info.major)")
PY_MINOR=$(python3 -c "import sys; print(sys.version_info.minor)")
if [ "$PY_MAJOR" -lt 3 ] || ([ "$PY_MAJOR" -eq 3 ] && [ "$PY_MINOR" -lt 7 ]); then
    fail "Python 3.7+ required, found ${PY_MAJOR}.${PY_MINOR}"
fi

# Download
info "Downloading admirarr..."
DOWNLOAD_URL="https://raw.githubusercontent.com/${REPO}/main/admirarr"
TMP_FILE=$(mktemp)
if command -v curl &>/dev/null; then
    curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE"
elif command -v wget &>/dev/null; then
    wget -qO "$TMP_FILE" "$DOWNLOAD_URL"
else
    fail "curl or wget required"
fi
ok "Downloaded"

# Install
chmod +x "$TMP_FILE"

# Try install dir, fall back to ~/.local/bin
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_FILE" "${INSTALL_DIR}/${BIN_NAME}"
    ok "Installed to ${INSTALL_DIR}/${BIN_NAME}"
else
    # Try with sudo
    if command -v sudo &>/dev/null; then
        info "Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "$TMP_FILE" "${INSTALL_DIR}/${BIN_NAME}"
        sudo chmod +x "${INSTALL_DIR}/${BIN_NAME}"
        ok "Installed to ${INSTALL_DIR}/${BIN_NAME}"
    else
        # Fall back to ~/.local/bin
        INSTALL_DIR="${HOME}/.local/bin"
        mkdir -p "$INSTALL_DIR"
        mv "$TMP_FILE" "${INSTALL_DIR}/${BIN_NAME}"
        ok "Installed to ${INSTALL_DIR}/${BIN_NAME}"

        # Check if in PATH
        if ! echo "$PATH" | grep -q "${INSTALL_DIR}"; then
            echo ""
            printf "  ${GOLD}!${RESET} Add this to your shell profile (~/.bashrc or ~/.zshrc):\n"
            printf "  ${DIM}export PATH=\"\${HOME}/.local/bin:\${PATH}\"${RESET}\n"
        fi
    fi
fi

# Verify
echo ""
if command -v admirarr &>/dev/null; then
    printf "${GREEN}✓${RESET} Ready! Run ${BOLD}admirarr help${RESET} to get started.\n"
else
    printf "  ${GREEN}✓${RESET} Installed! Open a new terminal or run:\n"
    printf "  ${DIM}export PATH=\"${INSTALL_DIR}:\${PATH}\"${RESET}\n"
    printf "  Then run ${BOLD}admirarr help${RESET} to get started.\n"
fi
echo ""
