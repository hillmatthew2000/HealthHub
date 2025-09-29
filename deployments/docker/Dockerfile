# Multi-stage build for smaller production image
FROM golang:1.21-alpine AS builder

# Set necessary environment variables
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Install git and ca-certificates for dependencies
RUN apk --no-cache add git ca-certificates tzdata

# Create appuser for security
RUN adduser -D -g '' appuser

WORKDIR /build

# Copy go mod and sum files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -ldflags="-w -s" -o main cmd/server/main.go

# Production stage
FROM scratch

# Import from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd /etc/passwd

# Copy the built binary
COPY --from=builder /build/main /app/

# Use non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/main", "-health-check"] || exit 1

# Run the application
ENTRYPOINT ["/app/main"]