# Stage 1: Build the Go binary
FROM golang:1.21 AS build

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./
# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the Go app statically
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /control-center .

# Stage 2: Create the final image
FROM gcr.io/distroless/static-debian11

# Copy the binary from the build stage
COPY --from=build /control-center /control-center

# Expose the port the control center listens on
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["/control-center"]
