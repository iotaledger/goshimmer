# syntax = docker/dockerfile:1.2.1

############################
# golang 1.17-buster amd64
FROM golang@sha256:cefedeae41e0bbbfa20bb1c37c3a43e0001fa541be9732f7bc6a28ecc154e9e4

# Set the current Working Directory inside the container
#RUN mkdir /tmp/goshimmer/tools/integration-tests/tester
WORKDIR /tmp/goshimmer/tools/integration-tests/tester

# Use Go Modules
COPY go.mod .
COPY go.sum .

ENV GO111MODULE=on
RUN go mod download
RUN go mod verify
