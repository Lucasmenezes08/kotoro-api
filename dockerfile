FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o kotoro-api ./cmd/api
RUN go build -o outbox-relay ./cmd/outbox-relay

FROM alpine:latest

WORKDIR /app

EXPOSE 8080

COPY --from=builder /app/kotoro-api ./kotoro-api
COPY --from=builder /app/outbox-relay ./outbox-relay

CMD ["./kotoro-api"]
