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

# define Environment Variable
ENV WSC_SERVER=127.0.0.1
ENV WSC_PORT=8990
ENV WSC_NUMBER_OFCLIENTS=10
# Float number smaller than 1.0 is smaller than 1per secnd and larger means the number of messages per second
# it is calculated as 1/WS_MESSAGES_PER_SECOND for the time delay between messages
ENV WSC_MESSAGES_PER_SECOND=1.0 
ENV WSC_LOG_LEVEL="debug"
ENV WSC_MAX_RECONNECT_ATTEMPT=10
# Delay in seconds
ENV WSC_DELAY_BETWEEN_RECONNECT=5

# Define the entrypoint for the container (the Go binary)
ENTRYPOINT ["./wsclient"]

# Expose any necessary ports (optional, depending on your app)
EXPOSE 8080

# Default command if no args are provided
CMD []
