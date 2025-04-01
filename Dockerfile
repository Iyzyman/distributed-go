# Use the official Go image as the build environment.
FROM golang:1.24-alpine AS builder

# Set the working directory.
WORKDIR /app

# Copy go.mod and go.sum first for dependency caching.
COPY go.mod ./
RUN go mod download

# Copy the rest of the source code.
COPY . .

# Build the server and client executables.
RUN go build -o server ./server
RUN go build -o client ./client

# Use a lightweight Alpine image for the final container.
FROM alpine:latest
WORKDIR /app

# Copy the built executables from the builder stage.
COPY --from=builder /app/server /app/server
COPY --from=builder /app/client /app/client

# Expose the UDP port (2222, as per your lab requirements).
EXPOSE 2222/udp

# Default command: start the server.
# Note: This can be overridden via docker-compose for client containers.
CMD ["./server/server", "--port=2222", "--semantics=at-most-once"]
