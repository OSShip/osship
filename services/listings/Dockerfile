FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY packages/ /app/packages/
COPY services/listings/ /app/services/listings/
WORKDIR /app/services/listings
RUN go mod download && CGO_ENABLED=0 go build -o /listings .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /listings /listings
EXPOSE 8082
CMD ["/listings"]
