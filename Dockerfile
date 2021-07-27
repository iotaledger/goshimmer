# syntax = docker/dockerfile:1.2.1

############################
# golang 1.16.3-buster amd64
FROM golang@sha256:dfa3cef088454200d6b48e2a911138f7d5d9afff77f89243eea6342f16ddcfb0 AS build

ARG BUILD_TAGS=rocksdb,builtin_static

# Ensure ca-certficates are up to date
RUN update-ca-certificates

# Set the current Working Directory inside the container
RUN mkdir /goshimmer
WORKDIR /goshimmer

# Use Go Modules
COPY go.mod .
COPY go.sum .

ENV GO111MODULE=on
RUN go mod download
RUN go mod verify

# 1. Mount everything from the current directory to the PWD(Present Working Directory) inside the container
# 2. Mount the build cache volume
# 3. Build the binary
# 4. Verify that goshimmer binary is statically linked
RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=linux GOARCH=amd64 go build \
    -tags="$BUILD_TAGS" \
    -ldflags='-w -s' \
    -o /go/bin/goshimmer

RUN wget -O /tmp/snapshot.bin https://dbfiles-goshimmer.s3.eu-central-1.amazonaws.com/snapshots/nectar/snapshot-latest.bin

############################
# Image
############################
# https://github.com/GoogleContainerTools/distroless/tree/master/cc
# using distroless cc image, which includes everything in the base image (glibc, libssl and openssl)
FROM gcr.io/distroless/cc@sha256:4cad7484b00d98ecb300916b1ab71d6c71babd6860c6c5dd6313be41a8c55adb

# Gossip
EXPOSE 14666/tcp
# Autopeering
EXPOSE 14626/udp
# FPC
EXPOSE 10895/tcp
# Pprof Profiling
EXPOSE 6061/tcp
# Prometheus exporter
EXPOSE 9311/tcp
# Webapi
EXPOSE 8080/tcp
# Dashboard
EXPOSE 8081/tcp

# Copy configuration
COPY --from=build /tmp/snapshot.bin /snapshot.bin
COPY config.default.json /config.json

# Copy the Pre-built binary file from the previous stage
COPY --chown=nonroot:nonroot --from=build /go/bin/goshimmer /run/goshimmer

ENTRYPOINT ["/run/goshimmer", "--config=/config.json", "--messageLayer.snapshot.file=/snapshot.bin", "--database.directory=/tmp/mainnetdb"]
