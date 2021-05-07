# syntax = docker/dockerfile:1.2.1

############################
# Build
############################
# golang:1.16.2-buster
FROM golang@sha256:a23a7e49a820f9ae69df0fedf64f037cb15b004997effa93ec885e5032277bc1 AS build

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
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /go/bin/goshimmer; \
    ./check_static.sh

############################
# Image
############################
# using static nonroot image
# user:group is nonroot:nonroot, uid:gid = 65532:65532
FROM gcr.io/distroless/static@sha256:23aa732bba4c8618c0d97c26a72a32997363d591807b0d4c31b0bbc8a774bddf

EXPOSE 14666/tcp
EXPOSE 14626/udp

# Copy configuration
COPY snapshot.bin /snapshot.bin
COPY config.default.json /config.json

# Copy the Pre-built binary file from the previous stage
COPY --from=build /go/bin/goshimmer /run/goshimmer

ENTRYPOINT ["/run/goshimmer", "--config=/config.json", "--messageLayer.snapshot.file=/snapshot.bin", "--database.directory=/tmp/mainnetdb"]
