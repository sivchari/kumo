FROM golang:1.26.8-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /kumo ./cmd/kumo

# Final image
FROM alpine:3.20

RUN apk add --no-cache ca-certificates \
    && addgroup -g 10000 -S kumo && adduser -u 10000 -S kumo -G kumo \
    && mkdir -p /data && chown kumo:kumo /data

COPY --from=builder /kumo /usr/local/bin/kumo

USER kumo

EXPOSE 4566

ENTRYPOINT ["kumo"]
CMD ["--host", "0.0.0.0", "--port", "4566"]
