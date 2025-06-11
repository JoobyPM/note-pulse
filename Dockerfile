# syntax=docker/dockerfile:1
ARG VERSION=dev

########################  builder  ##################################
FROM --platform=$BUILDPLATFORM ghcr.io/joobypm/note-pulse-builder:latest AS builder

# Import build platform args
ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG VERSION

ENV VERSION=${VERSION}

RUN apk add --no-cache git bash
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate swagger docs
RUN swag init -g ./docs/swagger.go --parseDependency --parseInternal

# Build binaries for the target platform
ENV GOOS=${TARGETOS:-linux}
ENV GOARCH=${TARGETARCH}
ENV CGO_ENABLED=0

# Echo VERSION
RUN echo "VERSION: ${VERSION}"
RUN echo "TARGETOS: ${TARGETOS}"
RUN echo "TARGETARCH: ${TARGETARCH}"

RUN chmod +x ./scripts/build.sh
RUN ./scripts/build.sh ./cmd/server main
RUN ./scripts/build.sh ./cmd/ping ping

# Final stage - smaller distroless
FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION
LABEL org.opencontainers.image.version=${VERSION} \
      org.opencontainers.image.source="https://github.com/joobypm/note-pulse"

WORKDIR /

COPY --from=builder /app/web-ui/ /web-ui/
COPY --from=builder /app/main .
COPY --from=builder /app/ping .

ENV VERSION=${VERSION}

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["./main"]
