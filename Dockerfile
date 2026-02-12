# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o familiar ./cmd/familiar

# Runtime stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates git docker-cli

COPY --from=builder /build/familiar /usr/local/bin/familiar

# Create directories with open permissions (actual dirs come from volume mounts)
RUN mkdir -p /etc/familiar /var/log/familiar /var/cache/familiar && \
    chmod 777 /var/log/familiar /var/cache/familiar

EXPOSE 7000

ENTRYPOINT ["familiar"]
CMD ["serve", "--config", "/etc/familiar/config.yaml"]
