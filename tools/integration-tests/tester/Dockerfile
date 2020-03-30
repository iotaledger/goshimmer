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


ENTRYPOINT ["go", "test", "./tests"]
