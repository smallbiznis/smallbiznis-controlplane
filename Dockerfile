# syntax=docker/dockerfile:1.7-labs

############################
# 1️⃣ Build Stage
############################
FROM golang:1.25-alpine AS build-stage

# Build environment
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Install minimal tools (for private repos, git etc)
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy entire repo
COPY . .

# Accept service path and binary name as args
ARG SERVICE_PATH
ARG OUTPUT_NAME

# Validate args
RUN test -n "$SERVICE_PATH" || (echo "❌ SERVICE_PATH not provided" && false)
RUN test -n "$OUTPUT_NAME" || (echo "❌ OUTPUT_NAME not provided" && false)

# Build binary
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /docker-gs-ping "./${SERVICE_PATH}"

############################
# 2️⃣ Release Stage (Distroless)
############################
FROM gcr.io/distroless/static:nonroot AS final

WORKDIR /app

# Copy built binary from build stage
COPY --from=build-stage "/docker-gs-ping" /docker-gs-ping

USER nonroot:nonroot

# Entrypoint
ENTRYPOINT ["/docker-gs-ping"]
