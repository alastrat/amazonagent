# Deployment Guide

## Quick Deploy (VPS with Docker)

### Prerequisites
- A VPS with Docker installed (Hetzner $5/mo, DigitalOcean $6/mo, AWS Lightsail $5/mo)
- Your `.env` file with all API keys configured
- Domain name (optional, for HTTPS)

### 1. Clone and configure

```bash
git clone <your-repo-url> fba-orchestrator
cd fba-orchestrator
cp .env.example .env
# Edit .env with your API keys:
# - ANTHROPIC_API_KEY
# - SP_API_LWA_APP_ID + SP_API_LWA_CLIENT_SECRET + SP_API_REFRESH_TOKEN
# - AMAZON_MERCHANT_TOKEN
# - EXA_API_KEY
# - TELEGRAM_BOT_TOKEN
# - DATABASE_URL (or use the Docker Postgres)
```

### 2. Start everything

```bash
# Without OpenFang (uses simulator — fast, free):
docker compose up -d

# With OpenFang (real LLM agents — requires Anthropic credits):
docker compose --profile openfang up -d
```

### 3. Verify

```bash
curl http://localhost:8081/health
# {"status":"ok"}
```

### Services

| Service | Port | URL |
|---------|------|-----|
| API | 8081 | http://your-server:8081 |
| Frontend | 3001 | http://your-server:3001 |
| Inngest | 8290 | http://your-server:8290 |
| Postgres | 54324 | Internal |
| OpenFang | 4200 | Internal (optional) |

### 4. Connect Telegram

Open Telegram → find @amazonagent_bot → send `/start`

### 5. Test

```bash
# Create a campaign
curl -X POST http://your-server:8081/campaigns \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{"type":"manual","trigger_type":"dashboard","criteria":{"keywords":["cast iron skillet"],"marketplace":"US","min_margin_pct":10}}'

# Upload a price list
curl -X POST http://your-server:8081/pricelist/upload \
  -H "Authorization: Bearer dev-token" \
  -F "file=@your_distributor_pricelist.csv"
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | Postgres connection string |
| `ANTHROPIC_API_KEY` | For LLM agents | Claude API key |
| `SP_API_LWA_APP_ID` | For real products | Amazon SP-API client ID |
| `SP_API_LWA_CLIENT_SECRET` | For real products | Amazon SP-API secret |
| `SP_API_REFRESH_TOKEN` | For real products | Amazon SP-API refresh token |
| `AMAZON_MERCHANT_TOKEN` | For eligibility | Your Amazon seller ID |
| `EXA_API_KEY` | For web search | Exa search API key |
| `FIRECRAWL_API_KEY` | For scraping | Firecrawl API key |
| `TELEGRAM_BOT_TOKEN` | For Telegram | Bot token from @BotFather |
| `OPENFANG_API_URL` | For real agents | Set to enable OpenFang (empty = simulator) |
| `INNGEST_DEV` | Dev only | Set to "1" for dev mode |

## Production Checklist

- [ ] Set `ENV=production` (disables dev auth)
- [ ] Configure real Supabase auth (JWT secret)
- [ ] Set up HTTPS (Caddy/nginx reverse proxy)
- [ ] Configure Inngest Cloud (replace dev server)
- [ ] Set up monitoring (health endpoint polling)
- [ ] Database backups (Supabase handles this)
- [ ] Rate limit the API
