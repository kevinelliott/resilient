#!/bin/bash
# scripts/run-two-nodes.sh

# Ensure we are running from the project root
cd "$(dirname "$0")/.."

echo "Building vaultd..."
go build -o bin/vaultd ./cmd/vaultd

echo "Starting Node A (port 8080 API, port 4001 P2P)..."
rm -f vaultA.db vaultA.db-wal vaultA.db-shm
./bin/vaultd -db vaultA.db -cas-dir vault_casA -api-port 8080 -p2p-port 4001 &
PID_A=$!

echo "Starting Node B (port 8081 API, port 4002 P2P)..."
rm -f vaultB.db vaultB.db-wal vaultB.db-shm
./bin/vaultd -db vaultB.db -cas-dir vault_casB -api-port 8081 -p2p-port 4002 &
PID_B=$!

echo "Both nodes are running."
echo "UI A is available at: http://127.0.0.1:8080/"
echo "UI B is available at: http://127.0.0.1:8081/"
echo "Press CTRL+C to stop both nodes."

# Catch CTRL+C and kill both nodes
trap "echo -e '\nShutting down nodes...'; kill $PID_A $PID_B; exit" INT TERM

wait $PID_A $PID_B
