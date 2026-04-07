#!/bin/sh
# Start a simple health endpoint on PORT (Railway's port) that proxies to OpenFang on 4200
PORT="${PORT:-4200}"

# If Railway sets a different PORT, create a health proxy
if [ "$PORT" != "4200" ]; then
  # Simple socat/nc proxy won't work without extra deps, so just start OpenFang
  # Override OpenFang to listen on Railway's PORT instead
  sed -i "s/api_listen = \"0.0.0.0:4200\"/api_listen = \"0.0.0.0:${PORT}\"/" /root/.openfang/config.toml
fi

exec openfang start
