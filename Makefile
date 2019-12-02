SHELL := /bin/bash
REPO := $(shell pwd)
PROTOC_GEN_GO := $(GOPATH)/bin/protoc-gen-go
# Protobuf generated go files
PROTO_FILES = $(shell find . -path ./vendor -prune -o -type f -name '*.proto' -print)
PROTO_GO_FILES = $(patsubst %.proto, %.pb.go, $(PROTO_FILES))

# If $GOPATH/bin/protoc-gen-go does not exist, we'll run this command to install it.
$(PROTOC_GEN_GO):
	(GO111MODULE=off go get -v github.com/golang/protobuf/protoc-gen-go)

# Implicit compile rule for GRPC/proto files
%.pb.go: %.proto | $(PROTOC_GEN_GO)
	protoc $< --go_out=plugins=grpc,paths=source_relative:.

.PHONY: compile
compile: $(PROTO_GO_FILES)
