# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o webhook ./cmd/webhook

# Final stage
FROM alpine:3.19 AS prod

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 webhook && \
    adduser -D -u 1000 -G webhook webhook

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/webhook .

# Change ownership
RUN chown -R webhook:webhook /app

# Switch to non-root user
USER webhook

# Expose webhook port
EXPOSE 8888

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8888/healthz || exit 1

# Run the webhook
ENTRYPOINT ["/app/webhook"]
