# 1. Build Stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod ./
# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the binary (Output name: "polaris")
RUN go build -o polaris cmd/server/main.go

# 2. Run Stage (Small image for production)
FROM alpine:latest

WORKDIR /root/
COPY --from=builder /app/polaris .

COPY --from=builder /app/frontend ./frontend

EXPOSE 8080

# Start the app
CMD ["./polaris"]