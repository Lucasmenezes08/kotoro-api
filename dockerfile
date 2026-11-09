FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o api ./cmd/api

FROM alpine:latest

WORKDIR /app

EXPOSE 8080

COPY --from=builder /app/api ./api

CMD ["./api"]
