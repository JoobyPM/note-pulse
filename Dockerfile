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


RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN swag init -g ./docs/swagger.go --parseDependency --parseInternal

# Build the application using shared build script
RUN chmod +x scripts/build.sh && \
    CGO_ENABLED=0 GOOS=linux ./scripts/build.sh ./cmd/server main

# Build the ping binary
RUN CGO_ENABLED=0 GOOS=linux ./scripts/build.sh ./cmd/ping ping

# Final stage - smaller distroless
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

# Copy the binary from builder stage
COPY --from=builder /app/main .
COPY --from=builder /app/ping .

# Use non-root user
USER nonroot:nonroot

# Expose port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./main"]
