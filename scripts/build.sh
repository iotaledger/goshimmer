#!/bin/bash
#
# Builds GoShimmer with the latest git tag and commit hash (short)
# E.g.: ./goshimmer -v --> GoShimmer 0.3.0-f3b76ae4

latest_tag=$(git describe --tags $(git rev-list --tags --max-count=1))
commit_hash=$(git rev-parse --short HEAD)

go build -ldflags="-s -w -X github.com/iotaledger/goshimmer/plugins/banner.AppVersion=${latest_tag:1}-$commit_hash" -tags rocksdb
