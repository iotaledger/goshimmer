#!/bin/bash
#
# Installs required dependencies and runs node's dashboard.

echo "::: Running /plugins/dashboard/frontend :::"
docker run -it --rm \
    -p 9999:9999 -u $(id -u ${USER}):$(id -g ${USER}) \
    --name="dashboard-dev-docker" \
    --volume="/etc/group:/etc/group:ro" \
    --volume="/etc/passwd:/etc/passwd:ro" \
    --volume="/etc/shadow:/etc/shadow:ro" \
    --network="docker-network_goshimmer" \
    -v $(pwd):/tmp/mnt \
    -e YARN_CACHE_FOLDER=/tmp/ \
    -e HOME=/tmp/ \
    -w /tmp/mnt/plugins/dashboard/frontend node:12.16 bash -c "yarn install && yarn start"
