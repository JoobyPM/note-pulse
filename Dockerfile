# syntax=docker/dockerfile:1
ARG VERSION=dev

########################  builder  ##################################
FROM ghcr.io/joobypm/note-pulse-builder:latest AS builder

ARG VERSION
ENV VERSION=${VERSION}          # forward to build.sh

RUN apk add --no-cache git bash
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN swag init -g ./docs/swagger.go --parseDependency --parseInternal

RUN chmod +x ./scripts/build.sh && ./scripts/build.sh ./cmd/server main
RUN ./scripts/build.sh ./cmd/ping ping

# Final stage - smaller distroless
FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION
LABEL org.opencontainers.image.version=${VERSION} \
      org.opencontainers.image.source="https://github.com/joobypm/note-pulse"

WORKDIR /

COPY --from=builder /app/main .
COPY --from=builder /app/ping .

ENV VERSION=${VERSION}

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["./main"]
