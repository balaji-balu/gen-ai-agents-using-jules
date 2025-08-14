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
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /agent .

# Stage 2: Create the final image
FROM gcr.io/distroless/static-debian11

# Copy the binary from the build stage
COPY --from=build /agent /agent

# Set the entrypoint
ENTRYPOINT ["/agent"]
