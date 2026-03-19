FROM golang:1.26-alpine AS builder

ARG VERSION=dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.version=${VERSION}" -o hookman ./cmd/hookman

FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl && \
    adduser -D -u 1000 appuser

WORKDIR /app
COPY --from=builder /app/hookman .
COPY --from=builder /app/migrations ./migrations

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 4000

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:4000/health || exit 1

LABEL org.opencontainers.image.title="hookman" \
      org.opencontainers.image.description="Self-hosted webhook delivery service" \
      org.opencontainers.image.source="https://github.com/david-ouatedem/hookman"

ENTRYPOINT ["./hookman"]
CMD ["serve"]
