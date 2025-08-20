#!/bin/bash

# Deploy Agent to remote host
set -e

REMOTE_HOST="ubuntu@20.205.180.232"
REMOTE_PATH="/usr/local/bin"
LOCAL_BUILD="bin/agent_v2"

# Ensure we run from repo root
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

# Generate version: datetime + short git sha (fallback: local)
VERSION="$(date +%Y.%m.%d-%H%M%S)-$(git rev-parse --short HEAD 2>/dev/null || echo local)"

echo "Building Agent for Linux, version: $VERSION..."
LD_FLAGS="-X main.Version=$VERSION"
GOOS=linux GOARCH=amd64 go build -ldflags "$LD_FLAGS" -o "$LOCAL_BUILD" ./cmd/agent_v2

echo "Stopping agent_v2 job before update..."
ssh $REMOTE_HOST "NOMAD_ADDR=http://localhost:4646 nomad job stop agent_v2"
echo "Waiting 3s for allocations to stop..."
sleep 3

echo "Uploading Agent to $REMOTE_HOST:$REMOTE_PATH..."
scp $LOCAL_BUILD $REMOTE_HOST:/tmp/agent_v2
ssh $REMOTE_HOST "sudo mv /tmp/agent_v2 $REMOTE_PATH/"

echo "Setting executable permissions..."
ssh $REMOTE_HOST "chmod +x $REMOTE_PATH/agent_v2"

echo "Restarting Agent job on remote host (fallback)..."
ssh $REMOTE_HOST "NOMAD_ADDR=http://localhost:4646 nomad job start agent_v2"


echo "Deploy completed successfully!"
echo "Agent is now available at $REMOTE_HOST:$REMOTE_PATH/agent_v2"