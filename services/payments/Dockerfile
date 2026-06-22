FROM rust:1.82-alpine AS builder
RUN apk add --no-cache musl-dev openssl-dev pkgconfig
WORKDIR /app
COPY services/payments/Cargo.toml services/payments/Cargo.lock* ./
RUN mkdir src && echo "fn main(){}" > src/main.rs && cargo build --release 2>/dev/null || true
COPY services/payments/src ./src
RUN cargo build --release

FROM alpine:3.20
RUN apk add --no-cache ca-certificates libgcc
COPY --from=builder /app/target/release/payments /payments
EXPOSE 8087
CMD ["/payments"]
