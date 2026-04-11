#!/bin/bash
set -e

# =============================================================================
# FBA Agent Orchestrator — Start All Services
# =============================================================================
# Fixed ports:
#   Postgres:  5433
#   Inngest:   8290 (connects to API at host:8081)
#   Go API:    8081
#   Next.js:   3001
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}Starting FBA Agent Orchestrator...${NC}"
echo ""

# --- Stop anything running on our ports ---
echo -e "${YELLOW}Cleaning up stale processes...${NC}"
lsof -ti:8081 | xargs kill -9 2>/dev/null || true
lsof -ti:3001 | xargs kill -9 2>/dev/null || true
docker stop inngest-local 2>/dev/null || true
docker rm inngest-local 2>/dev/null || true
docker compose down 2>/dev/null || true
sleep 1

# --- 1. Postgres ---
echo -e "${GREEN}[1/4] Starting Postgres on port 5433...${NC}"
PG_PORT=5433 docker compose up -d postgres
echo "     Waiting for Postgres to be healthy..."
for i in $(seq 1 30); do
  if docker compose ps postgres | grep -q "healthy"; then
    echo -e "     ${GREEN}Postgres ready.${NC}"
    break
  fi
  sleep 1
done

# --- 2. Inngest (pointing to host API) ---
echo -e "${GREEN}[2/4] Starting Inngest on port 8290...${NC}"
docker run -d --name inngest-local -p 8290:8288 \
  -e INNGEST_DEV=1 \
  inngest/inngest:latest \
  inngest dev -u "http://host.docker.internal:8081/api/inngest" \
  > /dev/null 2>&1
echo -e "     ${GREEN}Inngest ready.${NC}"

# --- 3. Go API ---
echo -e "${GREEN}[3/4] Starting Go API on port 8081...${NC}"
export PORT=8081
export DATABASE_URL="postgres://postgres:postgres@localhost:5433/fba_orchestrator?sslmode=disable"
export INNGEST_BASE_URL="http://127.0.0.1:8290"
export INNGEST_DEV=true
export INNGEST_EVENT_KEY=test
export INNGEST_SIGNING_KEY=""
export OPENFANG_API_URL=""
go run ./apps/api/main.go > /tmp/fba-api.log 2>&1 &
API_PID=$!
echo "$API_PID" > /tmp/fba-api.pid

# Wait for API to be ready
for i in $(seq 1 20); do
  if curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo -e "     ${GREEN}API ready (PID: $API_PID).${NC}"
    break
  fi
  sleep 1
done

if ! curl -s http://localhost:8081/health > /dev/null 2>&1; then
  echo -e "     ${RED}API failed to start. Check /tmp/fba-api.log${NC}"
  exit 1
fi

# --- 4. Next.js Frontend ---
echo -e "${GREEN}[4/4] Starting Next.js on port 3001...${NC}"
cd apps/web
NEXT_PUBLIC_API_URL=http://localhost:8081 npx next dev --port 3001 > /tmp/fba-web.log 2>&1 &
WEB_PID=$!
echo "$WEB_PID" > /tmp/fba-web.pid
cd "$PROJECT_DIR"

# Wait for frontend
for i in $(seq 1 30); do
  if curl -s http://localhost:3001 > /dev/null 2>&1; then
    echo -e "     ${GREEN}Frontend ready (PID: $WEB_PID).${NC}"
    break
  fi
  sleep 1
done

echo ""
echo -e "${GREEN}=============================================${NC}"
echo -e "${GREEN}  All services running!${NC}"
echo -e "${GREEN}=============================================${NC}"
echo ""
echo -e "  Frontend:  ${YELLOW}http://localhost:3001${NC}"
echo -e "  API:       ${YELLOW}http://localhost:8081${NC}"
echo -e "  Inngest:   ${YELLOW}http://localhost:8290${NC}"
echo -e "  Postgres:  ${YELLOW}localhost:5433${NC}"
echo ""
echo -e "  API logs:  /tmp/fba-api.log"
echo -e "  Web logs:  /tmp/fba-web.log"
echo ""
echo -e "  Stop all:  ${YELLOW}./scripts/stop.sh${NC}"
echo ""
