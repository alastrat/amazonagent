#!/bin/bash

# =============================================================================
# FBA Agent Orchestrator — Stop All Services
# =============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo -e "${RED}Stopping FBA Agent Orchestrator...${NC}"

# Stop Go API
if [ -f /tmp/fba-api.pid ]; then
  kill $(cat /tmp/fba-api.pid) 2>/dev/null && echo "  Stopped Go API" || true
  rm -f /tmp/fba-api.pid
fi
lsof -ti:8081 | xargs kill -9 2>/dev/null || true

# Stop Next.js
if [ -f /tmp/fba-web.pid ]; then
  kill $(cat /tmp/fba-web.pid) 2>/dev/null && echo "  Stopped Next.js" || true
  rm -f /tmp/fba-web.pid
fi
lsof -ti:3001 | xargs kill -9 2>/dev/null || true

# Stop Inngest
docker stop inngest-local 2>/dev/null && echo "  Stopped Inngest" || true
docker rm inngest-local 2>/dev/null || true

# Stop Docker Compose (Postgres)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"
docker compose down 2>/dev/null && echo "  Stopped Postgres" || true

echo ""
echo -e "${GREEN}All services stopped.${NC}"
