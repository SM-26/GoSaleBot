# Build stage
FROM golang:1.24.3-alpine AS builder

# Update apk and upgrade packages to reduce vulnerabilities
RUN apk update && apk upgrade --no-cache

# Set the working directory
WORKDIR /app

# Install build dependencies for CGO/SQLite
RUN apk add --no-cache gcc musl-dev sqlite

# Copy go.mod and go.sum first and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest of the source code
COPY . .

# Build the Go app and make it executable
RUN go build -o gosalebot
# RUN chmod +x gosalebot


# Final stage
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/gosalebot ./gosalebot

# Copy .env file into the container (optional, for reference or debugging)
COPY .env .env

# Export all environment variables from .env at container start
# Requires docker-compose or docker run with --env-file .env for actual env usage,
# but this line ensures .env is present and can be sourced if you use a shell entrypoint.
# If you want the Go app to see all .env vars, use docker-compose's env_file: .env

CMD ["./gosalebot"]