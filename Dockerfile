# Build stage
FROM golang:1.22-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy current directory contents into the container at /app
COPY . .

# Download and install any needed dependencies
RUN go mod download

# Build the Go app
RUN go build -o bot

# Final stage
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/bot ./bot

# If your bot needs config files or static assets, copy them here as well
# COPY --from=builder /app/config.yaml ./config.yaml

# Set environment variables if needed
# ENV TELEGRAM_TOKEN=your_token_here

# Command to run your app (replace with your actual entrypoint)
CMD ["./bot"]

# ---
# To customize for your app:
# - Change the base image if not using Go
# - Add build steps for your dependencies
# - Update the CMD to your app's entrypoint
