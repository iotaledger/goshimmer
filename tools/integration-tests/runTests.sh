#!/bin/bash

TEST_NAMES='autopeering common drng message value'

echo "Build GoShimmer image"
docker build -t iotaledger/goshimmer ../../.

echo "Pull additional Docker images"
docker pull angelocapossele/drand:latest
docker pull gaiaadm/pumba:latest
docker pull gaiadocker/iproute2:latest

for name in $TEST_NAMES
do
  TEST_NAME=$name docker-compose -f tester/docker-compose.yml up --abort-on-container-exit --exit-code-from tester --build
  docker logs tester &> logs/"$name"_tester.log
done

echo "Clean up"
docker-compose -f tester/docker-compose.yml down
docker rm -f $(docker ps -a -q -f ancestor=gaiadocker/iproute2)
