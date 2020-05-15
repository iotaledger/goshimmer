#!/usr/bin/env sh

[ -z "$TAG" ] && echo "TAG not set" >&2 && exit 1

# create docker volume and fix permissions
docker run --rm -v entrynode_db-"$TAG":/volume busybox chown -R 65532:65532 /volume
