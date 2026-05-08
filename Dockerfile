# Stage 1: Build React frontend
FROM node:22-alpine AS node-builder

WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.26-bookworm AS go-builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Embed the built frontend into the Go binary
COPY --from=node-builder /app/frontend/dist ./internal/static/dist

# Allow embedding version and commit via build args. Strip debug symbols for smaller binary.
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT}" -o /shakedown ./cmd/server/

# Stage 3: Runtime
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# audiowaveform is not in Debian bookworm repos; install pre-built .deb from GitHub
RUN curl -L -o /tmp/audiowaveform.deb \
    https://github.com/bbc/audiowaveform/releases/download/1.10.2/audiowaveform_1.10.2-1-12_amd64.deb \
    && apt-get update \
    && dpkg -i /tmp/audiowaveform.deb || true \
    && apt-get -f install -y --no-install-recommends \
    && rm /tmp/audiowaveform.deb \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user/group with UID/GID 1000
RUN addgroup --gid 1000 shakedown \
    && adduser --uid 1000 --gid 1000 --disabled-password --gecos '' shakedown

# Copy binary from go-builder
COPY --from=go-builder /shakedown /shakedown

# Create audio data directory and set ownership
RUN mkdir -p /data && chown 1000:1000 /data

# Declare data volume
VOLUME ["/data"]

USER 1000

# Set workdir after switching to non-root user
WORKDIR /home/shakedown

EXPOSE 8080

# Healthcheck against local server
HEALTHCHECK --interval=30s --timeout=5s \
    CMD curl -f http://localhost:8080/api/health || exit 1

CMD ["/shakedown"]
