FROM debian:bookworm-slim

ARG TARGETARCH

RUN apt-get update && apt-get install -y ca-certificates curl && rm -rf /var/lib/apt/lists/*

# Download pre-built binary from GitHub releases
RUN ARCH=$([ "$TARGETARCH" = "arm64" ] && echo "aarch64" || echo "x86_64") && \
    curl -fsSL "https://github.com/RightNow-AI/openfang/releases/download/v0.5.6/openfang-${ARCH}-unknown-linux-gnu.tar.gz" \
    | tar -xzf - -C /usr/local/bin/ && \
    chmod +x /usr/local/bin/openfang

RUN mkdir -p /root/.openfang /data

COPY config.toml /root/.openfang/config.toml
COPY start.sh /start.sh
RUN chmod +x /start.sh

EXPOSE 4200

CMD ["/start.sh"]
