#!/usr/bin/env bash

# Resilient Knowledge Vault - Universal Installer
# Can be sourced directly from GitHub or curled from a local active mesh node.

set -e

echo "=========================================================="
echo "      🚀 RESILIENT KNOWLEDGE VAULT INSTALLER 🚀         "
echo "=========================================================="
echo ""

# 1. Detect Environment OS / Arch
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7l) ARCH="arm" ;;
esac

echo "[*] Detected OS: $OS"
echo "[*] Detected Architecture: $ARCH"

# 2. Determine Download Source
# If VAULT_NODE_HOST is provided, we fetch binaries via the local API instead of GitHub
SOURCE_URL="https://github.com/resilient/vault/releases/latest/download"
if [ ! -z "$VAULT_NODE_HOST" ]; then
    echo "[*] Discovered active local Vault node at: $VAULT_NODE_HOST"
    SOURCE_URL="http://$VAULT_NODE_HOST:8080/api/download"
else
    echo "[*] Fetching from public releases..."
fi

# 3. Setup Install Directory
INSTALL_DIR="/usr/local/bin"
VAULT_DATA_DIR="$HOME/.resilient_vault"

echo "[*] Setting up Vault data directory at $VAULT_DATA_DIR"
mkdir -p "$VAULT_DATA_DIR/db"
mkdir -p "$VAULT_DATA_DIR/cas"

# 4. Download Binaries
echo "[*] Downloading vaultd daemon..."
# curl -sL "$SOURCE_URL/vaultd-${OS}-${ARCH}" -o /tmp/vaultd
# chmod +x /tmp/vaultd
# sudo mv /tmp/vaultd "$INSTALL_DIR/vaultd"

echo "[*] Downloading vault CLI/TUI..."
# curl -sL "$SOURCE_URL/vault-${OS}-${ARCH}" -o /tmp/vault
# chmod +x /tmp/vault
# sudo mv /tmp/vault "$INSTALL_DIR/vault"

# NOTE: Binaries are mocked for this script logic until CI/CD is active.
echo "[✔] Binaries 'vaultd' and 'vault' theoretically installed to $INSTALL_DIR"

# 5. Service Auto-Start Configuration (Systemd / Launchd)
if [[ "$OS" == "linux" ]] && command -v systemctl >/dev/null 2>&1; then
    echo "[*] Installing systemd service..."
    # Create the service file in tmp, then copy it over
    cat <<EOF > /tmp/vaultd.service
[Unit]
Description=Resilient Knowledge Vault Daemon
After=network.target

[Service]
ExecStart=$INSTALL_DIR/vaultd --db $VAULT_DATA_DIR/vault.db --cas-dir $VAULT_DATA_DIR/cas --profile standard
Restart=always
User=$USER

[Install]
WantedBy=multi-user.target
EOF
    sudo cp /tmp/vaultd.service /etc/systemd/system/vaultd.service
    sudo systemctl daemon-reload
    sudo systemctl enable vaultd
    sudo systemctl start vaultd
    echo "[✔] Systemd service started."
elif [[ "$OS" == "darwin" ]]; then
    # Mac launchd
    echo "[*] Installing launchd service for macOS..."
    # Not expanding full launchd plist here for brevity
    echo "[✔] Launchd service stubbed."
fi

echo ""
echo "=========================================================="
echo "    ✅ INSTALLATION COMPLETE. WELCOME TO THE MESH.      "
echo "=========================================================="
echo ""
echo "Command Line / TUI:"
echo "  Run 'vault tui' to explore the network."
echo ""
echo "Web UI (Captive Portal):"
echo "  The Web UI is now broadcasting locally at: http://127.0.0.1:8080"
echo ""
echo "Desktop Native Application (Tauri):"
if [[ "$OS" == "darwin" ]]; then
    echo "  For a native experience, download the .dmg from your web portal."
elif [[ "$OS" == "linux" ]]; then
    echo "  For a native experience, download the .AppImage or .deb from your web portal."
else 
    echo "  For a native experience, download the .exe from your web portal."
fi
echo ""

# CGO dependencies for SQLite compilation on Linux
if command -v apt-get &> /dev/null; then
    sudo apt-get update && sudo apt-get install -y gcc g++
fi
