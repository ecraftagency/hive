#!/bin/bash

# Build & deploy Server to remote host (no Nomad job control)
set -e

REMOTE_HOST="ubuntu@52.221.213.97"
REMOTE_PATH="/usr/local/bin"
LOCAL_BUILD="bin/server"

# Ensure running from repo root
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

echo "Building Server for Linux..."
GOOS=linux GOARCH=amd64 go build -o "$LOCAL_BUILD" ./cmd/server

echo "Uploading Server to $REMOTE_HOST:$REMOTE_PATH..."
scp "$LOCAL_BUILD" "$REMOTE_HOST:/tmp/server"
ssh "$REMOTE_HOST" "sudo mv /tmp/server $REMOTE_PATH/ && sudo chmod +x $REMOTE_PATH/server"

echo "Deploy completed successfully!"
echo "Server is now available at $REMOTE_HOST:$REMOTE_PATH/server"
