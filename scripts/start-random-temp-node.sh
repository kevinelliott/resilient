#!/bin/bash
# scripts/start-random-temp-node.sh

# Ensure we are running from the project root
cd "$(dirname "$0")/.."

echo "Building vaultd..."
go build -o bin/vaultd ./cmd/vaultd

# Find random available ports
find_port() {
    python3 -c 'import socket; s=socket.socket(); s.bind(("", 0)); print(s.getsockname()[1]); s.close()'
}

API_PORT=$(find_port)
P2P_PORT=$(find_port)

# Create a temporary directory for the workspace
TEMP_DIR=$(mktemp -d -t vaultd_XXXXXX)
DB_PATH="$TEMP_DIR/vault.db"
CAS_DIR="$TEMP_DIR/cas"

echo ""
echo "==========================================="
echo "🚀 Starting Ephemeral VaultD Node"
echo "==========================================="
echo "📂 Temp Workspace: $TEMP_DIR"
echo "🌐 API Port:       $API_PORT"
echo "📡 P2P Port:       $P2P_PORT"
echo "==========================================="
echo "UI is available at: http://127.0.0.1:$API_PORT/"
echo "Press CTRL+C to stop node and purge files."
echo ""

# Catch CTRL+C to cleanup the temporary directory
trap "echo -e '\n🧹 Shutting down node and purging temporary workspace...'; rm -rf \"$TEMP_DIR\"; kill 0; exit" INT TERM

# Start the node
./bin/vaultd -db "$DB_PATH" -cas-dir "$CAS_DIR" -api-port "$API_PORT" -p2p-port "$P2P_PORT" &

# Wait for the background process
wait
