# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /audiostreamer ./cmd/server

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ffmpeg ca-certificates tzdata

COPY --from=builder /audiostreamer /usr/local/bin/audiostreamer

EXPOSE 8080

CMD ["audiostreamer"]
