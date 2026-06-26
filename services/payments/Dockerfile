FROM rust:bookworm AS builder
RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake g++ pkg-config libssl-dev zlib1g-dev libcurl4-openssl-dev libsasl2-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY services/payments/Cargo.toml services/payments/Cargo.lock ./
# Cache dependency compilation only (stub main is replaced before the real build).
RUN mkdir src && echo 'fn main() {}' > src/main.rs && cargo build --release
COPY services/payments/src ./src
RUN cargo clean --package payments --release && cargo build --release
# Guard against shipping the dependency-cache stub binary (exits immediately with code 0).
RUN test "$(stat -c%s target/release/payments)" -gt 1000000

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libssl3 zlib1g libcurl4 libsasl2-2 wget \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/target/release/payments /payments
EXPOSE 8087
CMD ["/payments"]
