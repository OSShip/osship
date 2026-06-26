FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY packages/ /app/packages/
COPY services/gateway/ /app/services/gateway/
WORKDIR /app/services/gateway
RUN go mod download && CGO_ENABLED=0 go build -o /gateway .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
COPY --from=builder /gateway /gateway
EXPOSE 8080
CMD ["/gateway"]
