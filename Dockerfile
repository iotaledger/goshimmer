############################
# Build
############################
# golang:1.14.0-buster
FROM golang@sha256:fc7e7c9c4b0f6d2d5e8611ee73b9d1d3132750108878517bbf988aa772359ae4 AS build

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

# Copy everything from the current directory to the PWD(Present Working Directory) inside the container
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -ldflags='-w -s -extldflags "-static"' -a \
       -o /go/bin/goshimmer

############################
# Image
############################
# using static nonroot image
# user:group is nonroot:nonroot, uid:gid = 65532:65532
FROM gcr.io/distroless/static@sha256:23aa732bba4c8618c0d97c26a72a32997363d591807b0d4c31b0bbc8a774bddf

VOLUME /mainnetdb

EXPOSE 14666/tcp
EXPOSE 14626/udp

# Copy the Pre-built binary file from the previous stage
COPY --from=build /go/bin/goshimmer /run/goshimmer
# Copy the default config
COPY config.default.json /config.json

ENTRYPOINT ["/run/goshimmer", "--config-dir=/", "--database.directory=/mainnetdb"]
