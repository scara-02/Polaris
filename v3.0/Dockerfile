# Stage 1: Build
FROM golang:1.25-alpine AS builder
WORKDIR /app

# 1. Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# 2. Copy the entire source tree (needed for algo_ and internal/)
COPY . .

# 3. Use an argument to define which service to build
ARG SERVICE_NAME
RUN go build -o /bin/app ./cmd/${SERVICE_NAME}

# Stage 2: Final Image
FROM alpine:latest
WORKDIR /app
COPY --from=builder /bin/app /app/polaris_service

# Run the binary
CMD ["/app/polaris_service"]