# ─── Stage 1: Build ──────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# Install git (needed for go modules from VCS)
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src

# Cache dependency downloads separately from source compilation.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically-linked binary.
COPY . .
RUN VERSION=$(git describe --tags --always 2>/dev/null || echo "dev") && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /app/green-api-mcp-gateway ./cmd/server

# ─── Stage 2: Runtime ────────────────────────────────────────────────────────
FROM scratch

# Copy TLS certificates and timezone data from the builder stage so HTTPS
# calls and time-zone-aware logging work correctly in the minimal image.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the compiled binary.
COPY --from=builder /app/green-api-mcp-gateway /green-api-mcp-gateway

# Optional: copy the example config so users can mount their own over it.
COPY --from=builder /src/config/config.example.yaml /config/config.example.yaml

# stdio transport: the container reads from stdin / writes to stdout.
# For SSE/HTTP mode expose port 8090 and optionally 8091 (webhook receiver).
EXPOSE 8090 8091

ENTRYPOINT ["/green-api-mcp-gateway"]
