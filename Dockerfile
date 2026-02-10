# Build stage
FROM golang:1.20-alpine AS builder

WORKDIR /app

# Install git and gcc for CGO (SQLite support)
RUN apk add --no-cache git gcc musl-dev

# Copy go module files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary with SQLite support (CGO enabled)
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o modeltunnel ./cmd/modeltunnel/main.go

# Runtime stage (minimal)
FROM alpine:3.19

# Install required runtime dependencies
RUN apk --no-cache add ca-certificates sqlite

WORKDIR /app

# Create non-root user for security
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser

# Copy binary and default config from builder
COPY --from=builder /app/modeltunnel ./

# Create directories and default config
RUN mkdir -p /app/data /home/appuser/.config/modeltunnel && \
    chown -R appuser:appgroup /app /home/appuser/.config/modeltunnel

# Copy default config for Docker
COPY --chown=appuser:appgroup config.docker.yaml /home/appuser/.config/modeltunnel/config.yaml

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Set entrypoint to make it easy to run subcommands
ENTRYPOINT ["./modeltunnel"]
CMD ["--help"]