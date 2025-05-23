# --- Stage 1: Builder ---
# Use an official Go image as the base. Specify the Go version.
FROM golang:1.24.3-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Pre-copy/download modules for better layer caching
# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
# Download dependencies
RUN go mod download

# Copy the entire source code
COPY . .

# Build the Go application
# - CGO_ENABLED=0 produces a statically linked binary (important for minimal images like alpine/scratch)
# - ldflags="-w -s" removes debug information and symbols, reducing binary size
# -o specifies the output file name
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server cmd/server/main.go


# --- Stage 2: Runtime ---
# Use a minimal base image for the final stage
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Create a non-root user and group for security (optional but recommended)
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
# Consider changing ownership of necessary files/directories if needed
# RUN chown appuser:appgroup /app

# Copy only the compiled binary from the builder stage
COPY --from=builder /app/server /app/server


# Change to non-root user (optional but recommended)
USER appuser

# Expose the port the application listens on (matches config.yaml default)
EXPOSE 8080

# Define the entry point for the container
ENTRYPOINT ["/app/server"]