#!/bin/bash

TEST_NAMES='autopeering'

export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
echo "Build GoShimmer image"
docker build --build-arg DOWNLOAD_SNAPSHOT=0 -t iotaledger/goshimmer ../../.

echo "Pull additional Docker images"
docker pull angelocapossele/drand:v1.1.4
docker pull gaiaadm/pumba:0.7.2
docker pull gaiadocker/iproute2:latest

echo "Run integration tests"

for name in $TEST_NAMES; do
  TEST_NAME=$name docker-compose -f tester/docker-compose.yml up --abort-on-container-exit --exit-code-from tester --build
  docker logs tester &>logs/"$name"_tester.log
done

echo "Clean up"
docker-compose -f tester/docker-compose.yml down
docker rm -f $(docker ps -a -q -f ancestor=gaiadocker/iproute2)
