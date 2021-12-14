#!/bin/bash
#
# Installs required dependencies and runs node's dashboard.

echo "::: Running /plugins/dagsvisualizer/frontend :::"
docker run -it --rm \
    -p 3000:3000 -u $(id -u ${USER}):$(id -g ${USER}) \
    --name="dagsvisualizer-dev-docker" \
    --volume="/etc/group:/etc/group:ro" \
    --volume="/etc/passwd:/etc/passwd:ro" \
    --volume="/etc/shadow:/etc/shadow:ro" \
    --network="docker-network_shimmer" \
    -v $(pwd):/tmp/mnt \
    -e YARN_CACHE_FOLDER=/tmp/ \
    -e HOME=/tmp/ \
    -w /tmp/mnt/plugins/dagsvisualizer/frontend node:17.2 bash -c "yarn install && yarn start"