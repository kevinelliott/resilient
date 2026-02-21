# Stage 1: Build the React web frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# Stage 2: Build the Go backend daemon and CLI
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build static binaries to avoid C library dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/vaultd ./cmd/vaultd
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/vault ./cmd/vault

# Stage 3: Create the final minimal production image
FROM alpine:latest
WORKDIR /app

# Install root certs for potential API/HTTPS egress
RUN apk add --no-cache ca-certificates

# Copy compiled frontend assets (daemon serves from ./web/dist)
COPY --from=frontend-builder /app/web/dist ./web/dist

# Copy the binaries to the PATH
COPY --from=backend-builder /app/bin/vaultd /usr/local/bin/vaultd
COPY --from=backend-builder /app/bin/vault /usr/local/bin/vault

# Make a volume mount point for resilient data if needed
VOLUME /app/data

# API & Kademlia DHT / P2P Swarm Swarm Ports
EXPOSE 8080 4001

# Entrypoint daemon
ENTRYPOINT ["vaultd"]
