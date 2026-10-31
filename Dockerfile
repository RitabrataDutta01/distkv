# Stage 1: Build the binary
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install git and dependencies if needed
RUN apk add --no-cache git

# Copy dependency files first to cache module downloads
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary statically
RUN CGO_ENABLED=0 GOOS=linux go build -o distkv-server ./cmd/server

# Stage 2: Runtime Environment
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for secure networking if needed
RUN apk add --no-cache ca-certificates

# Copy the compiled binary from the builder stage
COPY --from=builder /app/distkv-server /app/distkv-server

# Expose your server port (adjust if your config uses a different port)
EXPOSE 8080

# Run the server binary
ENTRYPOINT ["/app/distkv-server"]
