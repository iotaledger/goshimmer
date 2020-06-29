############################
# Build
############################
# golang:1.14.4-buster
FROM golang@sha256:fbaba67d3bd0a6fd154eaa27d1a0a9e5e80ecdb0792736017fde7326d9bf8d69 AS build

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

EXPOSE 14666/tcp
EXPOSE 14626/udp

# Copy configuration
COPY snapshot.bin /snapshot.bin
COPY config.default.json /config.json

# Copy the Pre-built binary file from the previous stage
COPY --from=build /go/bin/goshimmer /run/goshimmer

ENTRYPOINT ["/run/goshimmer", "--config-dir=/", "--valueLayer.snapshot.file=/snapshot.bin", "--database.directory=/tmp/mainnetdb"]
