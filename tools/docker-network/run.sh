#!/bin/bash

# Create a function to join an array of strings by a given character
function join { local IFS="$1"; shift; echo "$*"; }

# All parameters can be optional now, just make sure we don't have too many
if [[ $# -gt 3 ]] ; then
    echo 'Call with ./run [replicas=1|2|3|...] [grafana=0|1] [drng=0|1]'
    exit 0
fi

REPLICAS=${1:-1}
GRAFANA=${2:-0}
DRNG=${3:-0}

export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
echo "Build GoShimmer"
# Allow docker compose to build and cache an image
docker-compose build

# check exit code of builder
if [ $? -ne 0 ]
then
  echo "Building failed. Please fix and try again!"
  exit 1
fi

echo "Run GoShimmer network"
# SHIMMER_PEER_REPLICAS is used in docker-compose.yml to determine how many replicas to create
export SHIMMER_PEER_REPLICAS=$REPLICAS
# Profiles is created to set which docker profiles to run
# https://docs.docker.com/compose/profiles/
PROFILES=()
if [ $GRAFANA -ne 0 ]
then
  PROFILES+=("grafana")
fi

if [ $DRNG -ne 0 ]
then
  PROFILES+=("drng")
fi
export COMPOSE_PROFILES=$(join , ${PROFILES[@]})
docker-compose up

echo "Clean up docker resources"
docker-compose down -v