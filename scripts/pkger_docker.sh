#!/bin/bash
#
# Builds both the node's and the analysis dashboard and runs pkger afterwards.

echo "::: Building /plugins/dashboard/frontend :::"
docker run -it --rm \
    -u $(id -u ${USER}):$(id -g ${USER}) \
    --volume="/etc/group:/etc/group:ro" \
    --volume="/etc/passwd:/etc/passwd:ro" \
    --volume="/etc/shadow:/etc/shadow:ro" \
    -v $(pwd):/tmp/mnt \
    -e YARN_CACHE_FOLDER=/tmp/ \
    -e HOME=/tmp/ \
    -w /tmp/mnt/plugins/dashboard/frontend node:12.16 bash -c "yarn install && yarn build"

echo "::: Building /plugins/analysis/dashboard/frontend :::"
docker run --rm -it \
    -u $(id -u ${USER}):$(id -g ${USER})  \
    --volume="/etc/group:/etc/group:ro" \
    --volume="/etc/passwd:/etc/passwd:ro" \
    --volume="/etc/shadow:/etc/shadow:ro" \
    -v $(pwd):/tmp/mnt \
    -e YARN_CACHE_FOLDER=/tmp/ \
    -e HOME=/tmp/ \
    -w /tmp/mnt/plugins/analysis/dashboard/frontend node:12.16 bash -c  "yarn install && yarn build"

echo "::: Running /plugins/dagsvisualizer/frontend :::"
docker run -it --rm \
    -u $(id -u ${USER}):$(id -g ${USER}) \
    --volume="/etc/group:/etc/group:ro" \
    --volume="/etc/passwd:/etc/passwd:ro" \
    --volume="/etc/shadow:/etc/shadow:ro" \
    -v $(pwd):/tmp/mnt \
    -e YARN_CACHE_FOLDER=/tmp/ \
    -e HOME=/tmp/ \
    -w /tmp/mnt/plugins/dagsvisualizer/frontend node:17.2 bash -c "yarn install && yarn build"

echo "::: Running pkger :::"
docker run --rm -it \
    -u $(id -u ${USER}):$(id -g ${USER})  \
    --volume="/etc/group:/etc/group:ro" \
    --volume="/etc/passwd:/etc/passwd:ro" \
    --volume="/etc/shadow:/etc/shadow:ro" \
    -v $(pwd):/tmp/mnt \
    -e YARN_CACHE_FOLDER=/tmp/ \
    -e HOME=/tmp/ \
    -w /tmp/mnt/ \
    golang:1.18-rc bash -c "go install github.com/markbates/pkger/cmd/pkger@latest && pkger"
