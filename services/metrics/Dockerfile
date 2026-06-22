FROM rust:1.82-alpine AS builder
RUN apk add --no-cache musl-dev openssl-dev pkgconfig cmake make g++
WORKDIR /app
COPY services/metrics/Cargo.toml services/metrics/Cargo.lock* ./
RUN mkdir src && echo "fn main(){}" > src/main.rs && cargo build --release 2>/dev/null || true
COPY services/metrics/src ./src
RUN cargo build --release

FROM alpine:3.20
RUN apk add --no-cache ca-certificates libgcc libstdc++
COPY --from=builder /app/target/release/metrics /metrics
EXPOSE 8088
CMD ["/metrics"]
