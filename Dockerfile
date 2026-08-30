# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Stage 1 — build
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Dependencies are copied and downloaded on their own layer so a source-only
# change doesn't re-download the module cache.
COPY go.mod go.sum ./
ENV GOPROXY=https://package-mirror.liara.ir/repository/go/
RUN go mod download

COPY . .

# CGO_ENABLED=0  -> fully static binary, runs on any base image
# -trimpath      -> strips local filesystem paths from the binary
# -ldflags "-s -w" -> drops the symbol table and DWARF data (~25% smaller)
# The cache mounts make rebuilds a few seconds instead of a full recompile.
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/admin ./cmd/admin

# ---------------------------------------------------------------------------
# Stage 2 — runtime
# ---------------------------------------------------------------------------
# Alpine (not scratch/distroless) so the image keeps a shell for debugging and
# wget for the container healthcheck. For a hardened deployment, swap this for
# gcr.io/distroless/static-debian12:nonroot and drop the HEALTHCHECK.
FROM alpine:3.20 AS runtime

# ca-certificates: required for any outbound HTTPS call.
# tzdata: the app also embeds it via `import _ "time/tzdata"`, but having it on
# disk means `docker exec date` and Postgres client tools behave sensibly.
RUN apk add --no-cache ca-certificates tzdata wget && \
    adduser -D -u 10001 -h /app appuser

WORKDIR /app

COPY --from=builder /out/api   /app/api
COPY --from=builder /out/admin /app/admin

USER appuser

EXPOSE 8080

# Hits the liveness endpoint, which touches no dependencies — so a slow database
# never marks the container itself unhealthy.
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

# Exec form (not shell form) so the process is PID 1 and receives SIGTERM
# directly — this is what makes graceful shutdown actually work in Docker.
ENTRYPOINT ["/app/api"]
