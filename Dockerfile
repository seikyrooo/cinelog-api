# Stage 1: Build the Go application
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Enable automatic Go toolchain resolution if needed
ENV GOTOOLCHAIN=auto

# Install git and certificates
RUN apk add --no-cache git ca-certificates tzdata

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o cinelog-api main.go

# Stage 2: Lightweight runtime image
FROM alpine:3.21

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/cinelog-api .

# Create uploads & data directories
RUN mkdir -p /app/uploads/posters /app/uploads/backdrops /app/uploads/avatars /app/data

EXPOSE 3000

VOLUME ["/app/uploads", "/app/data"]

CMD ["./cinelog-api"]