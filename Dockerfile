FROM golang:1.25.7-trixie AS builder
ENV ROOT=/build
ARG BUILD_TAGS=""
RUN mkdir ${ROOT}
WORKDIR ${ROOT}

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    GOOS=linux go build -o shutdown-manager -tags $BUILD_TAGS $ROOT && chmod +x ./shutdown-manager

FROM debian:13.7-slim@sha256:f6b08b09a05725f9431502d3b5714f5c4576e82c1a84fa1debe0c24a7e0c3ec4
WORKDIR /app

RUN rm -f /etc/apt/apt.conf.d/docker-clean; echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' > /etc/apt/apt.conf.d/keep-cache
RUN --mount=type=cache,target=/var/lib/apt,sharing=locked \
    --mount=type=cache,target=/var/cache/apt,sharing=locked \
    apt-get -y update && apt-get upgrade -y && apt-get install -y ca-certificates systemd-sysv

COPY --from=builder /build/shutdown-manager ./
EXPOSE 8080

ENTRYPOINT ["./shutdown-manager"]
