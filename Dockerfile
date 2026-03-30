# Builder stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install swag tool
RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Generate Swagger docs during build
RUN swag init -g pkg/api/api.go

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /app/main ./cmd

# Runtime stage
FROM alpine:latest

# Install ca-certificates (needed for HTTPS requests even though we aren't using them yet and timezone data
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the compiled binary from builder stage
COPY --from=builder /app/main .

# Copy the seed file
COPY --from=builder /app/appointments.json .
COPY --from=builder /app/.env .
COPY --from=builder /app/docs ./docs

# Business logic in the app handles this, but we set it here for consistency
ENV TZ=America/Los_Angeles

# Expose the port your app listens on
EXPOSE 8080

# Run the application
CMD ["./main"]