# Build stage
FROM golang:1.24.3-alpine AS builder

# Update apk and upgrade packages to reduce vulnerabilities
RUN apk update && apk upgrade --no-cache

# Set the working directory
WORKDIR /app

# Copy current directory contents into the container at /app
COPY . .

# Download and install any needed dependencies
RUN go mod download

# Install SQLite dependency for Go
RUN apk add --no-cache sqlite

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
ENV MODERATION_GROUP_ID=${MODERATION_GROUP_ID}
ENV APPROVED_GROUP_ID=${APPROVED_GROUP_ID}
ENV TIMEOUT_MINUTES=${TIMEOUT_MINUTES}
ENV LANG=${LANG}

# Command to run your app (replace with your actual entrypoint)
CMD ["./bot"]

# ---
# To customize for your app:
# - Change the base image if not using Go
# - Add build steps for your dependencies
# - Update the CMD to your app's entrypoint
