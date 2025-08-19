#!/bin/bash

# Build & deploy Web client to remote host (no Nomad job control)
set -e

REMOTE_HOST="ubuntu@52.221.213.97"
REMOTE_PATH="/usr/local/bin"
LOCAL_BUILD="bin/web"

# Ensure running from repo root
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

echo "Building Web client for Linux..."
GOOS=linux GOARCH=amd64 go build -o "$LOCAL_BUILD" ./cmd/web

echo "Stopping web job before update..."
ssh $REMOTE_HOST "NOMAD_ADDR=http://localhost:4646 nomad job stop web"
echo "Waiting 3s for allocations to stop..."
sleep 3

echo "Uploading Web client to $REMOTE_HOST:$REMOTE_PATH..."
scp "$LOCAL_BUILD" "$REMOTE_HOST:/tmp/web"
ssh "$REMOTE_HOST" "sudo mv /tmp/web $REMOTE_PATH/ && sudo chmod +x $REMOTE_PATH/web"

echo "Restarting Web job on remote host (fallback)..."
ssh $REMOTE_HOST "NOMAD_ADDR=http://localhost:4646 nomad job start web"

echo "Deploy completed successfully!"
echo "Web client is now available at $REMOTE_HOST:$REMOTE_PATH/web"
