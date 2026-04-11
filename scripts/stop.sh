#!/bin/bash

# =============================================================================
# FBA Agent Orchestrator — Stop All Services
# Does NOT stop Docker/Postgres — use `make down` for that
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

# Stop Inngest container (but NOT Postgres)
docker stop inngest-local 2>/dev/null && echo "  Stopped Inngest" || true
docker rm inngest-local 2>/dev/null || true

echo ""
echo -e "${GREEN}All app services stopped. Docker/Postgres still running.${NC}"
echo -e "To stop everything including Docker: ${RED}make down${NC}"
