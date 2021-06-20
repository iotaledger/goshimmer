#!/bin/bash

set -x

TEST_NAMES='tipsel'

export DOCKER_BUILDKIT=1

GOSHIMMER_BASE=$PWD/../..

SWARM_MANAGER="65.21.49.191"
SWARM_MANAGER_USER="deployer"
export SWARM_ENDPOINT="ssh://$SWARM_MANAGER_USER@$SWARM_MANAGER:22"
export SWARM_REGISTRY="127.0.0.1:5000"

ssh -f -n -N -o ConnectTimeout=3 $SWARM_MANAGER_USER@$SWARM_MANAGER -L $SWARM_REGISTRY:$SWARM_REGISTRY

if [ $? != 0 ]; then
  echo "Unable to connect to Swarm Manager"
  exit -1
fi

VANILLA_IMAGE_NAME=$SWARM_REGISTRY/disintegration-vanilla
pushd $GOSHIMMER_BASE
docker build -t $VANILLA_IMAGE_NAME .
docker push $VANILLA_IMAGE_NAME
popd

echo "Run DISintegration tests"

for name in $TEST_NAMES; do

  unset DOCKER_HOST

  ##########################
  # Prepare patched images
  ##########################
  pushd tester/tests/$name
  for quilt in quilt_*; do
    BUILD_DIR=$(mktemp -d)
    cp -a $GOSHIMMER_BASE $BUILD_DIR/goshimmer
    pushd $BUILD_DIR/*
    for patch in tools/disintegration-tests/tester/tests/$name/$quilt/*.patch; do
      patch -p1 <$patch
    done
    IMAGE_NAME=$SWARM_REGISTRY/disintegration-$name-$quilt
    docker build -t $IMAGE_NAME .
    docker push $IMAGE_NAME
    popd
    rm -rf $BUILD_DIR
  done

  ##########################
  # Prepare tester image
  ##########################
  BUILD_DIR=$(mktemp -d)
  cp -a $GOSHIMMER_BASE $BUILD_DIR/goshimmer
  pushd $BUILD_DIR/*
  grep -v tools .dockerignore >.dockerignore_
  mv .dockerignore_ .dockerignore
  TESTER_IMAGE_NAME=$SWARM_REGISTRY/disintegration-$name-tester
  docker build -t $TESTER_IMAGE_NAME --build-arg test_name=$name -f tools/disintegration-tests/tester/Dockerfile .
  docker push $TESTER_IMAGE_NAME
  popd
  rm -rf $BUILD_DIR

  ##########################
  # We run stuff remotely
  ##########################
  export DOCKER_HOST=$SWARM_ENDPOINT
  TESTER_SERVICE=$(docker service create --mount type=bind,source=/var/run/docker.sock,destination=/var/run/docker.sock,readonly=true --mode replicated-job -d $TESTER_IMAGE_NAME)
  TESTER_CONTAINER=$(docker service ps --format '{{.Name}}.{{.ID}}' --no-trunc $TESTER_SERVICE)
  popd
  docker service logs $TESTER_SERVICE &>logs/"$name"_tester.log

done
