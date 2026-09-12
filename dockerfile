FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o api ./cmd/api

FROM alpine:3.22

WORKDIR /app

EXPOSE 8080

COPY --from=builder /app/api ./api

CMD ["./api"]
