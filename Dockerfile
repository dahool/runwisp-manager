# syntax=docker/dockerfile:1.7
ARG RUNWISP_VERSION=1.7.0

FROM node:22.23.2-bookworm-slim AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN bun run build

FROM golang:1.27.0-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/build ./internal/ui/dist
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o /out/runwisp-manager ./cmd/runwisp-manager

FROM runwisp/runwisp:${RUNWISP_VERSION} AS runwisp

FROM debian:bookworm-slim
ARG RWM_UID=1000
ARG RWM_GID=1000
RUN apt-get update \
 && apt-get install -y --no-install-recommends git openssh-client ca-certificates tini \
 && rm -rf /var/lib/apt/lists/* \
 && groupadd -o -g ${RWM_GID} rwm \
 && useradd -o -u ${RWM_UID} -g ${RWM_GID} -m -d /home/rwm -s /usr/sbin/nologin rwm \
 && install -d -o ${RWM_UID} -g ${RWM_GID} -m 0700 /var/lib/runwisp-manager
COPY --from=runwisp /usr/local/bin/runwisp /usr/local/bin/runwisp
COPY --from=build /out/runwisp-manager /usr/local/bin/runwisp-manager
COPY deploy/docker/known_hosts.github /etc/runwisp-manager/known_hosts.github
ENV RWM_LISTEN=0.0.0.0:9478 \
    RWM_STATE_DIR=/var/lib/runwisp-manager \
    RWM_RUNWISP_DATA_DIR=/var/lib/runwisp \
    RWM_RUNWISP_BIN=/usr/local/bin/runwisp \
    RWM_STRICT_VERSION=true
USER ${RWM_UID}:${RWM_GID}
EXPOSE 9478
VOLUME ["/var/lib/runwisp-manager"]
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 CMD ["runwisp-manager", "healthcheck"]
ENTRYPOINT ["/usr/bin/tini", "--", "runwisp-manager"]
CMD ["serve"]
