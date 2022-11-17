#!/bin/bash
#
# Builds both the node's and the analysis dashboard.

echo "::: Building /plugins/dashboard/frontend :::"
rm -rf plugins/dashboard/frontend/build

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
rm -rf plugins/analysis/dashboard/frontend/build

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
rm -rf plugins/dagsvisualizer/frontend/build

docker run -it --rm \
    -u $(id -u ${USER}):$(id -g ${USER}) \
    --volume="/etc/group:/etc/group:ro" \
    --volume="/etc/passwd:/etc/passwd:ro" \
    --volume="/etc/shadow:/etc/shadow:ro" \
    -v $(pwd):/tmp/mnt \
    -e YARN_CACHE_FOLDER=/tmp/ \
    -e HOME=/tmp/ \
    -w /tmp/mnt/plugins/dagsvisualizer/frontend node:17.2 bash -c "yarn install && yarn build"
