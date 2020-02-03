SHELL := /bin/bash
REPO := $(shell pwd)
GOFILES_NOVENDOR := $(shell go list -f "{{.Dir}}" ./...)
PACKAGES_NOVENDOR := $(shell go list ./...)
PROTOC_GEN_GO := $(GOPATH)/bin/protoc-gen-go

# Protobuf generated go files
PROTO_FILES = $(shell find . -path ./vendor -prune -o -type f -name '*.proto' -print)
PROTO_GO_FILES = $(patsubst %.proto, %.pb.go, $(PROTO_FILES))
PROTO_GO_FILES_REAL = $(shell find . -path ./vendor -prune -o -type f -name '*.pb.go' -print)

.PHONY: build
build: proto
	go build -o goshimmer

# Protobuffing
.PHONY: proto
proto: $(PROTO_GO_FILES)

# If $GOPATH/bin/protoc-gen-go does not exist, we'll run this command to install it.
$(PROTOC_GEN_GO):
	(GO111MODULE=off go get -v github.com/golang/protobuf/protoc-gen-go)

# Implicit compile rule for GRPC/proto files
%.pb.go: %.proto | $(PROTOC_GEN_GO)
	protoc $< --go_out=plugins=grpc,paths=source_relative:.

.PHONY: clean_proto
clean_proto:
	@rm -f $(PROTO_GO_FILES_REAL)

.PHONY: vet
vet:
	@echo "Running go vet."
	@go vet ${PACKAGES_NOVENDOR}

.PHONY: test
test: vet
	go test -timeout 30s ./... ${GOPACKAGES_NOVENDOR}
