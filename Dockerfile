# Build Stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 creates a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -o omnibridge cmd/server/main.go

# Runtime Stage
FROM alpine:latest

WORKDIR /root/

# Install ca-certificates for HTTPS calls (needed for Gemini API)
RUN apk --no-cache add ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /app/omnibridge .

# Copy necessary directories for runtime
COPY --from=builder /app/agents ./agents
COPY --from=builder /app/seeds ./seeds

# Create storage directory for persistence
RUN mkdir -p storage

# Expose the default server port
EXPOSE 8080

# Set default environment variables (can be overridden)
ENV MODE=server
ENV ADDR=:8080
ENV PROVIDER=gemini
ENV MODEL=gemini-2.0-flash

# Default command to run the server
CMD ["./omnibridge", "--mode", "server", "--addr", ":8080"]
