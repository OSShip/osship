FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY packages/ /app/packages/
COPY services/notifications/ /app/services/notifications/
WORKDIR /app/services/notifications
RUN go mod download && CGO_ENABLED=0 go build -o /notifications .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /notifications /notifications
EXPOSE 8086
CMD ["/notifications"]
