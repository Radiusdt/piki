# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o vector-dsp ./cmd/vector-dsp

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates and tzdata
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /build/vector-dsp /app/vector-dsp

# Copy static files if any
COPY --from=builder /build/static /app/static 2>/dev/null || true

# Create data directory
RUN mkdir -p /app/data

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run
ENTRYPOINT ["/app/vector-dsp"]
