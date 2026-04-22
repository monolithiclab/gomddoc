# Stage 1: Build
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Cache dependencies first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically-linked binary with stripped debug info
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /gomddoc ./cmd/gomddoc

# Stage 2: Runtime (distroless for minimal attack surface)
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /gomddoc /gomddoc

EXPOSE 8080

ENTRYPOINT ["/gomddoc"]
