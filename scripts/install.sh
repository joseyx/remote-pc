#!/usr/bin/env bash
# ==============================================================================
# Remote PC - Automated Installer for macOS & Linux
# ==============================================================================
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${CYAN}${BOLD}"
echo "====================================================="
echo "       Remote PC - Unix Installer (macOS & Linux)    "
echo "====================================================="
echo -e "${NC}"

OS="$(uname -s)"
ARCH="$(uname -m)"
echo -e "${BLUE}==>${NC} Sistema detectado: ${BOLD}${OS} (${ARCH})${NC}"

# Target installation directory
INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "${INSTALL_DIR}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN_TARGET="${INSTALL_DIR}/remotepc-server"

# 1. Compile or locate binary
if [ -f "${ROOT_DIR}/server/cmd/server/main.go" ]; then
    if command -v go >/dev/null 2>&1; then
        echo -e "${BLUE}==>${NC} Compilando remotepc-server con Go..."
        mkdir -p "${ROOT_DIR}/bin"
        (cd "${ROOT_DIR}/server" && go build -ldflags="-s -w" -o "${ROOT_DIR}/bin/remotepc-server" ./cmd/server)
        cp "${ROOT_DIR}/bin/remotepc-server" "${BIN_TARGET}"
    else
        echo -e "${RED}Error:${NC} No se encontró 'go' instalado para compilar el servidor."
        if [ -f "${ROOT_DIR}/bin/remotepc-server" ]; then
            echo -e "${BLUE}==>${NC} Usando binario preexistente en bin/remotepc-server..."
            cp "${ROOT_DIR}/bin/remotepc-server" "${BIN_TARGET}"
        else
            echo "Por favor instala Go (https://go.dev/doc/install) o descarga un binario compilado."
            exit 1
        fi
    fi
elif [ -f "./remotepc-server" ]; then
    cp "./remotepc-server" "${BIN_TARGET}"
else
    echo -e "${RED}Error:${NC} No se encontró el código fuente ni el binario 'remotepc-server'."
    exit 1
fi

chmod +x "${BIN_TARGET}"
echo -e "${GREEN}✓${NC} Binario instalado en: ${BOLD}${BIN_TARGET}${NC}"

# 2. Add ~/.local/bin to PATH if not already present
PATH_EXPORT='export PATH="$HOME/.local/bin:$PATH"'
SHELL_CONFIGS=()

if [ -f "${HOME}/.zshrc" ]; then
    SHELL_CONFIGS+=("${HOME}/.zshrc")
fi
if [ -f "${HOME}/.bashrc" ]; then
    SHELL_CONFIGS+=("${HOME}/.bashrc")
fi
if [ -f "${HOME}/.profile" ] && [ ${#SHELL_CONFIGS[@]} -eq 0 ]; then
    SHELL_CONFIGS+=("${HOME}/.profile")
fi

# Fallback default if no rc file existed yet
if [ ${#SHELL_CONFIGS[@]} -eq 0 ]; then
    if [ "${OS}" = "Darwin" ]; then
        SHELL_CONFIGS+=("${HOME}/.zshrc")
    else
        SHELL_CONFIGS+=("${HOME}/.bashrc")
    fi
fi

UPDATED_PATH=false
for CFG in "${SHELL_CONFIGS[@]}"; do
    if ! grep -q '\.local/bin' "${CFG}" 2>/dev/null; then
        echo "" >> "${CFG}"
        echo "# Remote PC Server CLI" >> "${CFG}"
        echo "${PATH_EXPORT}" >> "${CFG}"
        echo -e "${GREEN}✓${NC} Se agregó ~/.local/bin al PATH en: ${CFG}"
        UPDATED_PATH=true
    fi
done

# 3. Configure MCP (Claude Desktop and Claude Code CLI)
echo -e "${BLUE}==>${NC} Configurando integración MCP para Claude Code y Claude Desktop..."
"${BIN_TARGET}" setup-mcp || true

echo -e "\n${GREEN}${BOLD}====================================================="
echo " ✓ ¡Instalación de Remote PC completada con éxito!"
echo "=====================================================${NC}"
echo -e "Puedes iniciar Remote PC ejecutando:"
echo -e "  ${BOLD}remotepc-server${NC}"
echo ""
if [ "${UPDATED_PATH}" = true ]; then
    echo -e "${CYAN}Nota:${NC} Para usar el comando inmediatamente en esta terminal, ejecuta:"
    if [ "${OS}" = "Darwin" ]; then
        echo -e "  ${BOLD}source ~/.zshrc${NC}"
    else
        echo -e "  ${BOLD}source ~/.bashrc${NC}"
    fi
fi
echo ""
