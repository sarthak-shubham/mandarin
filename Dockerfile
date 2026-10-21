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
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o tamarind ./cmd/tamarind

# Stage 2: Minimal runtime image
FROM alpine:3.20

WORKDIR /app

# Copy CA certificates for HTTPS webhook dispatching
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary and default configuration
COPY --from=builder /app/tamarind /usr/local/bin/tamarind
COPY --from=builder /app/tamarind.yaml /etc/tamarind/tamarind.yaml

# Expose HTTP port
EXPOSE 8080

# Run Tamarind
ENTRYPOINT ["tamarind"]
CMD ["-config", "/etc/tamarind/tamarind.yaml"]
