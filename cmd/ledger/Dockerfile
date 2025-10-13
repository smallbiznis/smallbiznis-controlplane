# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.25-alpine AS build-stage

# Set environment variables
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

# Copy ONLY go.mod and go.sum first
COPY go.mod go.sum ./

RUN go mod download

# Copy entire source tree
COPY . .

# Build binary from specific service
RUN go build -ldflags="-s -w" -o /docker-gs-ping ./cmd/ledger

# Final stage (distroless or alpine)
FROM alpine:latest AS build-release-stage

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary from build-stage
COPY --from=build-stage /docker-gs-ping /docker-gs-ping

# USER nonroot:nonroot

ENTRYPOINT ["/docker-gs-ping"]
