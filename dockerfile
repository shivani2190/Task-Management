# Use official Golang image as build stage
FROM golang:1.23 AS builder

# Set working directory inside container
WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod tidy

# Copy the source code
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .

# Use a minimal image for the final stage
FROM scratch

# Copy the built binary from the builder stage
COPY --from=builder /app/main /main

# Copy the .env file (if needed)
COPY --from=builder /app/.env /.env

# Expose the application port
EXPOSE 8080

# Run the application
CMD ["/main"]
