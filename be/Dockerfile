# Step 1: Build the Go application
FROM golang:latest AS build

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod tidy

# Copy the entire application source code
COPY . .

# Set CGO_ENABLED=0 to build a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Step 2: Create the production image
FROM alpine:latest

# Set working directory
WORKDIR /root/

# Install dependencies required to run the Go binary (like certificates for HTTPS)
RUN apk add --no-cache ca-certificates

# Copy the built Go binary from the build image
COPY --from=build /app/main .

# Copy the .env file into the container
COPY --from=build /app/.env .env

# Expose port 8080 for the backend
EXPOSE 8080

# Run the Go application
CMD ["./main"]
