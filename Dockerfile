# Build stage
FROM golang:1.25-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY src/ ./src/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o unifi-protect-mqtt ./src

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create a non-root user
RUN adduser -D app

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/unifi-protect-mqtt .

# Change ownership to non-root user
RUN chown app:app unifi-protect-mqtt

# Switch to non-root user
USER app

# Run the application
CMD ["./unifi-protect-mqtt"]