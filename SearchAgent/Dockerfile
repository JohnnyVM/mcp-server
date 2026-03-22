FROM golang:latest AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o searchagent-mcp .

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends chromium ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /build/searchagent-mcp /usr/local/bin/searchagent-mcp
COPY sources.json /app/sources.json
ENV SOURCES_FILE=/app/sources.json
ENTRYPOINT ["searchagent-mcp"]
