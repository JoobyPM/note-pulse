# Build stage
FROM golang:1.24.2-alpine AS builder

# Install git for version info
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with same ldflags as Makefile
RUN set -x && \
    REV=$(git rev-parse --short HEAD 2>/dev/null || echo "none") && \
    TAG=$(git describe --tags --dirty --always 2>/dev/null || echo "dev") && \
    BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ') && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath \
        -ldflags "-s -w -X main.version=$TAG -X main.commit=$REV -X main.builtAt=$BUILD_TIME" \
        -o main ./cmd/server

# Final stage - distroless
FROM gcr.io/distroless/static:nonroot

WORKDIR /

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Use non-root user
USER nonroot:nonroot

# Expose port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./main"]
