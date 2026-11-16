# Multi-stage build for Go backend
FROM golang:1.25.1-alpine AS builder

WORKDIR /app

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with static linking
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o main .

# Final stage - minimal runtime image
FROM alpine:3.22

# Install ca-certificates for OAuth HTTPS calls
RUN apk --no-cache add ca-certificates && addgroup -S app && adduser -S -G app -H app

WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/main .

USER app

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1

CMD ["./main"]
