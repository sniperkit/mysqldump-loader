#!/bin/sh
set -x
set -e

env GOOS=linux go build -v ./docker/mysqldump-loader-linux
docker build --force-rm -t sniperkit/mysqldump-loader:3.7-alpine --no-cache -f ./docker/dockerfile-alpine3.7