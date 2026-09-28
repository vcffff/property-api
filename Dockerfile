FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

RUN apk add --no-cache ca-certificates && adduser -D appuser
USER appuser

WORKDIR /app
COPY --from=builder /app/server ./server

EXPOSE 8080

ENTRYPOINT ["./server"]
