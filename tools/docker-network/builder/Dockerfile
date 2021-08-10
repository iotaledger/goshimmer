ARG GOLANG_IMAGE_TAG=1.16.7-alpine

FROM golang:${GOLANG_IMAGE_TAG} as build

ARG BUILD_TAGS=rocksdb,builtin_static,netgo
ARG BINARY_NAME=goshimmer
ARG WORK_DIR=/go/src/goshimmer

ENV GO111MODULE=on

WORKDIR ${WORK_DIR}

COPY . .
RUN apk update && apk add build-base
RUN go mod download 
RUN	go mod verify 
RUN go build -tags ${BUILD_TAGS} -ldflags '-w -s' -o ${BINARY_NAME} main.go pkged.go

FROM golang:${GOLANG_IMAGE_TAG} as final

ARG BINARY_NAME=goshimmer
ARG WORK_DIR=/go/src/goshimmer

RUN apk update && apk add build-base
COPY --from=build ${WORK_DIR}/${BINARY_NAME} /go/bin/

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

ENTRYPOINT [ "/go/bin/goshimmer" ]
