# syntax=docker/dockerfile:1

# ---- Stage 1: build the React frontend ---------------------------------------
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
# Vite writes into ../server/frontend/dist (embedded by the Go build).
RUN mkdir -p ../server/frontend/dist && npm run build

# ---- Stage 2: build a static Go binary ------------------------------------------
FROM golang:1.26-alpine AS server
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/server/frontend/dist ./server/frontend/dist
ARG VERSION=dev
RUN go build -ldflags "-s -w -X github.com/novapixel1010/family-holocron/internal/version.Version=${VERSION}" \
      -o /out/holocron ./bin/holocron

# ---- Stage 3: minimal runtime -----------------------------------------------------
# No package installs here: CA certificates come from the builder and time zone
# data is embedded in the binary (time/tzdata), so this stage needs no network.
FROM alpine:3.20
COPY --from=server /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN addgroup -S -g 10001 holocron \
 && adduser -S -D -H -u 10001 -G holocron holocron \
 && mkdir -p /var/opt/holocron/data /var/opt/holocron/media \
 && chown -R holocron:holocron /var/opt/holocron
COPY --from=server /out/holocron /usr/local/bin/holocron
USER holocron
ENV HOLOCRON_PORT=8080 \
    HOLOCRON_DATA_DIR=/var/opt/holocron/data \
    HOLOCRON_MEDIA_DIR=/var/opt/holocron/media
EXPOSE 8080
VOLUME ["/var/opt/holocron/data", "/var/opt/holocron/media"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/usr/local/bin/holocron", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/holocron"]
CMD ["serve"]
