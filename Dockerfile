# syntax=docker/dockerfile:1

# Stage 1: Build React frontend
FROM node:22-alpine@sha256:8ea2348b068a9544dae7317b4f3aafcdc032df1647bb7d768a05a5cad1a7683f AS node-builder

WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# Stage 2: Build audiowaveform from source (not packaged in Alpine)
FROM alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d AS aw-builder

RUN apk add --no-cache \
    cmake make g++ git \
    boost-dev libmad-dev libid3tag-dev libsndfile-dev gd-dev flac-dev

ARG AUDIOWAVEFORM_VERSION=1.10.2
RUN git clone --depth 1 --branch ${AUDIOWAVEFORM_VERSION} \
    https://github.com/bbc/audiowaveform.git /tmp/aw

WORKDIR /tmp/aw/build
RUN cmake -DCMAKE_INSTALL_PREFIX=/usr -DENABLE_TESTS=0 .. && make -j$(nproc)

# Stage 3: Build Go binary
FROM golang:1.26-alpine@sha256:91eda9776261207ea25fd06b5b7fed8d397dd2c0a283e77f2ab6e91bfa71079d AS go-builder

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

# Stage 4: Runtime
FROM alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d

LABEL org.opencontainers.image.source="https://github.com/aaronkyriesenbach/shakedown" \
      org.opencontainers.image.description="Self-hosted recording library"

# Install ffmpeg, ca-certificates, and audiowaveform runtime deps
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    boost1.84-filesystem \
    boost1.84-program_options \
    libmad \
    libid3tag \
    libsndfile \
    gd \
    flac-libs

COPY --from=aw-builder /tmp/aw/build/audiowaveform /usr/bin/audiowaveform

# Create non-root user/group with UID/GID 1000
RUN addgroup -g 1000 shakedown \
    && adduser -u 1000 -G shakedown -D -g '' shakedown

# Copy binary from go-builder
COPY --from=go-builder --chmod=755 --chown=1000:1000 /shakedown /shakedown

RUN mkdir -p /data && chown 1000:1000 /data

USER 1000
WORKDIR /home/shakedown

EXPOSE 8080

# Healthcheck uses the built-in subcommand instead of curl
HEALTHCHECK --interval=30s --timeout=5s \
    CMD ["/shakedown", "healthcheck"]

CMD ["/shakedown"]
