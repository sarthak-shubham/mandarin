# Stage 1: Build the binary
FROM golang:alpine AS builder

WORKDIR /app

# Install ca-certificates for secure outbound HTTPS webhooks
RUN apk --no-cache add ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o mochi ./cmd/mochi

# Stage 2: Minimal runtime image
FROM alpine:3.20

WORKDIR /app

# Copy CA certificates for HTTPS webhook dispatching
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary and default configuration
COPY --from=builder /app/mochi /usr/local/bin/mochi
COPY --from=builder /app/mochi.yaml /etc/mochi/mochi.yaml

# Expose HTTP port
EXPOSE 8080

# Run Mochi
ENTRYPOINT ["mochi"]
CMD ["-config", "/etc/mochi/mochi.yaml"]
