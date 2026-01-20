# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git and other build dependencies if needed (not needed for pure go sqlite, but good practice)
# RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the binary
# CGO_ENABLED=0 is default for pure Go, but good to be explicit.
# modernc.org/sqlite is pure Go, so CGO_ENABLED=0 works fine.
RUN CGO_ENABLED=0 GOOS=linux go build -o osrs-events cmd/osrs-events/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests (Discord/Firebase)
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/osrs-events .

# Create a directory for data persistence
RUN mkdir -p /app/data

# Set environment variable for database path to point to the data volume
ENV DATABASE_PATH=/app/data/osrs_events.db

# Expose port if you add a web server later (optional)
# EXPOSE 8080

CMD ["./osrs-events"]
