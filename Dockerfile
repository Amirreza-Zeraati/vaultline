# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

FROM golang:1.26-alpine AS builder

WORKDIR /src

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG VERSION=dev

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    GOOS=linux \
    go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/api \
      ./cmd/api


# ---------------------------------------------------------------------------
# Runtime
# ---------------------------------------------------------------------------

FROM alpine:3.20

#RUN apk add --no-cache \
#      ca-certificates \
#      tzdata \
#      wget \
#    && adduser \
#      -D \
#      -u 10001 \
#      -h /app \
#      appuser

RUN adduser \
      -D \
      -u 10001 \
      -h /app \
      appuser

WORKDIR /app

COPY --from=builder /out/api /app/api

USER appuser

EXPOSE 8080

HEALTHCHECK \
  --interval=15s \
  --timeout=3s \
  --start-period=10s \
  --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/api"]