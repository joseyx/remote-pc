#!/usr/bin/env bash
# ==============================================================================
# Orbit - Automated Uninstaller for macOS & Linux
# ==============================================================================
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${CYAN}${BOLD}"
echo "====================================================="
echo "       Orbit - Unix Uninstaller (macOS & Linux)       "
echo "====================================================="
echo -e "${NC}"

INSTALL_DIR="${HOME}/.local/bin"
BIN_TARGET="${INSTALL_DIR}/orbit"
LEGACY_TARGET="${INSTALL_DIR}/remotepc-server"

# 1. Uninstall Daemon/Service if installed
if [ -x "${BIN_TARGET}" ]; then
    echo -e "${BLUE}==>${NC} Desinstalando servicio en segundo plano si está activo..."
    "${BIN_TARGET}" service uninstall 2>/dev/null || true
    echo -e "${BLUE}==>${NC} Removiendo configuración MCP de Claude..."
    "${BIN_TARGET}" uninstall-mcp 2>/dev/null || true
fi

# 2. Remove binaries
if [ -f "${BIN_TARGET}" ] || [ -L "${BIN_TARGET}" ]; then
    rm -f "${BIN_TARGET}"
    echo -e "${GREEN}✓${NC} Removido binario: ${BIN_TARGET}"
fi

if [ -f "${LEGACY_TARGET}" ] || [ -L "${LEGACY_TARGET}" ]; then
    rm -f "${LEGACY_TARGET}"
    echo -e "${GREEN}✓${NC} Removido alias: ${LEGACY_TARGET}"
fi

echo -e "\n${GREEN}${BOLD}====================================================="
echo "   ✓ Desinstalación de Orbit completada."
echo "=====================================================${NC}"
echo ""
