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
    -w /tmp/mnt/plugins/dashboard/frontend node:12.16 bash -c "yarn install"

echo "::: Running /plugins/dashboard/frontend :::"
docker run -it --name goshimmer-dashboard-dev --rm \
  -p 9093:9090  -u $(id -u ${USER}):$(id -g ${USER}) \
    --volume="/etc/group:/etc/group:ro" \
    --volume="/etc/passwd:/etc/passwd:ro" \
    --volume="/etc/shadow:/etc/shadow:ro" \
    -v $(pwd):/tmp/mnt \
    -e YARN_CACHE_FOLDER=/tmp/ \
    -e HOME=/tmp/ \
    -w /tmp/mnt/plugins/dashboard/frontend node:12.16 yarn start
