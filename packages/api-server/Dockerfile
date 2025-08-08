# Build stage
# Using Debian-based Go image for playwright.Run() compatibility
FROM golang:1.24-bookworm AS builder

# Build arguments for version information
ARG VERSION=dev
ARG COMMIT_ID=unknown
ARG BUILD_TIME=unknown
ARG MODULE_PREFIX=github.com/babelcloud/gbox
ARG VERSION_PKG=${MODULE_PREFIX}/packages/api-server/internal/misc/service

RUN go env -w GOCACHE=/go-cache
RUN go env -w GOMODCACHE=/gomod-cache

# Install the Playwright Go CLI tool binary
RUN --mount=type=cache,target=/gomod-cache --mount=type=cache,target=/go-cache \
    go install github.com/playwright-community/playwright-go/cmd/playwright@v0.5200.0

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download Go dependencies
RUN --mount=type=cache,target=/gomod-cache go mod download

# Copy source code
COPY . .

# Build the application with version information
RUN --mount=type=cache,target=/gomod-cache --mount=type=cache,target=/go-cache \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X ${VERSION_PKG}.Version=${VERSION} \
    -X ${VERSION_PKG}.BuildTime=${BUILD_TIME} \
    -X ${VERSION_PKG}.CommitID=${COMMIT_ID}" \
    -o api-server ./cmd/app/main.go

# --- Final stage ---
# Switch to Debian Slim for glibc compatibility needed by playwright.Run() internals
FROM debian:bookworm-slim

# Install Node.js/npm (needed to RUN the playwright CLI tool for driver install) 
# and essential runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Set timezone
ENV TZ=Asia/Shanghai

# Copy the Playwright CLI tool binary built in the builder stage
# The Go builder image typically puts binaries in /go/bin
COPY --from=builder /go/bin/playwright /usr/local/bin/playwright

# Install playwright drivers
RUN playwright help

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/api-server .

# Copy config files if any
COPY --from=builder /app/config ./config

# Expose port
EXPOSE 28080

# Run the application
# Playwright will auto-download its small drivers to ~/.cache/ms-playwright-go on first run
CMD ["./api-server"] 