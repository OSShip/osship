FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY packages/ /app/packages/
COPY services/sessions/ /app/services/sessions/
WORKDIR /app/services/sessions
RUN go mod download && CGO_ENABLED=0 go build -o /sessions .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /sessions /sessions
EXPOSE 8084
CMD ["/sessions"]
