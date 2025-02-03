# syntax=docker/dockerfile:1

#####################################################
# Stage 1: Build the Go application
#####################################################
FROM golang:1.23.5 AS builder

# Create an app directory
WORKDIR /app

# Copy the module files first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the server executable
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o /server main.go

#####################################################
# Stage 2: Create a lightweight runtime image
#####################################################
FROM alpine:3.14

# Add a non-root user for better security
RUN addgroup -S temporal && adduser -S temporal -G temporal

WORKDIR /app
COPY --from=builder /server /app/server

# Switch to non-root user
USER temporal

# Expose port 8081 (matching the port in main.go)
EXPOSE 8081

ENTRYPOINT ["/app/server"] 