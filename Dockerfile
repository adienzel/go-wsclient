# Step 1: Build the Go binary (large image)
FROM golang:1.23 AS builder

# Set the working directory inside the container
WORKDIR /app

# Clone the Go source code from GitHub (replace with your repo URL)
RUN git clone https://github.com/adienzel/go-wsclient.git .

# Install necessary Go dependencies (modules)
RUN go mod tidy

# Build the Go application binary
RUN go build -o wsclient .

# Step 2: Build a minimal image with just the compiled executable
FROM alpine:latest

# Install any necessary dependencies (if required by your Go binary)
# RUN apk add --no-cache ca-certificates

# Set the working directory in the final image
WORKDIR /root/

# Copy the compiled Go binary from the build stage
COPY --from=builder /app/wsclient .

# Make the binary executable
RUN chmod +x wsclient
# Define the entrypoint for the container (the Go binary)
ENTRYPOINT ["./wsclient"]

# Expose any necessary ports (optional, depending on your app)
EXPOSE 8080

# Default command if no args are provided
CMD ["-server", "http://localhost:8890", "-clients", "2", "-rate", "1.0", "-version", "V1.0", "-loglevel", "debug",]
