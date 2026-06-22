FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY packages/ /app/packages/
COPY services/users/ /app/services/users/
WORKDIR /app/services/users
RUN go mod download && CGO_ENABLED=0 go build -o /users .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /users /users
EXPOSE 8083
CMD ["/users"]
