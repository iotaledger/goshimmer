FROM golang:1.16

ARG test_name

COPY . /goshimmer

WORKDIR /goshimmer/tools/disintegration-tests/tester

ENV TEST_NAME=$test_name

CMD /goshimmer/tools/disintegration-tests/assets/entrypoint.sh
