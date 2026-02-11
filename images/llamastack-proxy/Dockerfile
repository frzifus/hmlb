# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o llamastack-proxy .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/llamastack-proxy .

# Expose the proxy port
EXPOSE 8322

# Set default environment variables
ENV LLAMASTACK_URL=http://localhost:8321
ENV PROXY_PORT=8322

# Run the proxy
CMD ["./llamastack-proxy"]
