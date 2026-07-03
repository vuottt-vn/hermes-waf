# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./

# Copy source code
COPY . .

# Tidy and download dependencies
RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /waf ./cmd/waf

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' wafuser

# Copy binary and configs
COPY --from=builder /waf /app/waf
COPY configs /app/configs

# Create log directory and certs directory
RUN mkdir -p /var/log/vinahost-waf /app/certs && \
    chown -R wafuser:wafuser /var/log/vinahost-waf /app/certs && \
    chmod -R 755 /app/configs

# Switch to non-root user
USER wafuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
ENTRYPOINT ["/app/waf"]
CMD ["-config", "/app/configs/config.yaml"]
