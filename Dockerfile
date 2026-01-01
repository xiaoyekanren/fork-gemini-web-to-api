# Build stage
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git build-base

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o ai-bridges cmd/server/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy the binary from the builder
COPY --from=builder /app/ai-bridges .

# Copy example config
COPY config.example.yml config.example.yml

# Create an empty config.yml if it doesn't exist (optional, or just expect mount)
# RUN touch config.yml

# Expose the port the app runs on
EXPOSE 3000

# Command to run the application
CMD ["./ai-bridges"]
