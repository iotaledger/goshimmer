#!/bin/bash

echo "Build GoShimmer image"
docker build -t iotaledger/goshimmer ../../.

echo "Pull additional Docker images"
docker pull angelocapossele/drand:latest
docker pull gaiaadm/pumba:latest
docker pull gaiadocker/iproute2:latest

echo "Run integration tests"
docker-compose -f tester/docker-compose.yml up --abort-on-container-exit --exit-code-from tester --build

echo "Create logs from containers in network"
docker logs tester &> logs/tester.log

echo "Clean up"
docker-compose -f tester/docker-compose.yml down
docker rm -f $(docker ps -a -q -f ancestor=gaiadocker/iproute2)
