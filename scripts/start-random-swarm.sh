#!/bin/bash
# scripts/start-random-swarm.sh

NUM_NODES=${1:-10}

# Ensure we are running from the project root
cd "$(dirname "$0")/.."

echo "Building vaultd..."
go build -o bin/vaultd ./cmd/vaultd

PIDS=""
TEMP_DIRS=""

echo "Starting $NUM_NODES random nodes..."

for i in $(seq 1 $NUM_NODES); do
    API_PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("", 0)); print(s.getsockname()[1]); s.close()')
    P2P_PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("", 0)); print(s.getsockname()[1]); s.close()')
    TEMP_DIR=$(mktemp -d -t vaultd_XXXXXX)
    TEMP_DIRS="$TEMP_DIRS $TEMP_DIR"
    DB_PATH="$TEMP_DIR/vault.db"
    CAS_DIR="$TEMP_DIR/cas"
    
    ./bin/vaultd -db "$DB_PATH" -cas-dir "$CAS_DIR" -api-port "$API_PORT" -p2p-port "$P2P_PORT" >/dev/null 2>&1 &
    PID=$!
    PIDS="$PIDS $PID"
    echo "Node $i started. API: http://127.0.0.1:$API_PORT"
done

echo ""
echo "All $NUM_NODES nodes running."
echo "Press CTRL+C to stop all nodes and purge all $NUM_NODES workspaces."

cleanup() {
    echo -e '\n🧹 Shutting down swarm and purging temporary workspaces...'
    kill $PIDS 2>/dev/null
    rm -rf $TEMP_DIRS
    exit
}

trap cleanup INT TERM

wait $PIDS
