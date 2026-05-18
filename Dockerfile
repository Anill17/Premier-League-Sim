# syntax=docker/dockerfile:1
# ============================================================
# Multi-stage build for league-api.
# Stage 1 compiles the binary with the Go toolchain.
# Stage 2 packages it into a minimal scratch-style image.
# ============================================================

FROM golang:1.22-alpine AS builder
WORKDIR /src

COPY . .

# Resolve any missing checksums (first build) then compile statically
# so the final image has no glibc dependency.
ENV CGO_ENABLED=0
RUN go mod tidy && \
    go build -trimpath -ldflags="-s -w" -o /out/league-api ./cmd/server

# ------------------------------------------------------------

FROM alpine:3.20
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=builder /out/league-api /app/league-api
USER app

EXPOSE 8080
ENTRYPOINT ["/app/league-api"]
