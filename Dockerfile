# we need to use alpine to build since cgo is required
FROM golang:1.13-alpine AS build
RUN apk add --no-cache git gcc g++

# Set the current Working Directory inside the container
RUN mkdir /goshimmer
WORKDIR /goshimmer

# Download dependencies
COPY go.mod . 
COPY go.sum .
RUN go mod download

# Copy everything from the current directory to the PWD(Present Working Directory) inside the container
COPY . .

# Build
RUN CGO_ENABLED=1 GOOS=linux go build -o /go/bin/goshimmer

FROM alpine:latest  

RUN apk --no-cache add ca-certificates

WORKDIR /app

VOLUME /app/mainnetdb

EXPOSE 14666/tcp
EXPOSE 14626/udp
EXPOSE 14626/tcp

# Copy the Pre-built binary file from the previous stage
COPY --from=build /go/bin/goshimmer .
# Copy the docker config
COPY docker.config.json config.json

ENTRYPOINT ["./goshimmer"] 
