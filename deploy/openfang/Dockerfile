FROM rust:1.87-bookworm AS builder

RUN apt-get update && apt-get install -y pkg-config libssl-dev && rm -rf /var/lib/apt/lists/*

WORKDIR /src
RUN git clone --depth 1 https://github.com/RightNow-AI/openfang.git .

# Use debug profile to avoid LTO memory exhaustion, then strip the binary
ENV CARGO_PROFILE_RELEASE_LTO=false
ENV CARGO_PROFILE_RELEASE_CODEGEN_UNITS=16
RUN cargo build --release -p openfang-cli

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates curl && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/target/release/openfang /usr/local/bin/openfang

RUN mkdir -p /root/.openfang /data

EXPOSE 4200

CMD ["openfang", "start"]
