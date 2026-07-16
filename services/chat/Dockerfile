FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the service binary
RUN CGO_ENABLED=0 GOOS=linux go build -o chat-service ./services/chat/cmd

# Final deployment stage
FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/chat-service .
COPY --from=builder /app/.env* ./

EXPOSE 8084

CMD ["./chat-service"]
