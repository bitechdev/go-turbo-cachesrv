#!/bin/sh


docker build . --build-arg CACHEBUST=$(date +%s) -t bitech/go-turbo-cachesrv
docker stop go-turbo-cachesrv
docker rm go-turbo-cachesrv
docker volume create --name go-turbo-cachesrv

docker run -d -p 8080:8080 -e TURBO_AUTH_TOKEN=test -v go-turbo-cachesrv:/data --name go-turbo-cachesrv --restart unless-stopped --memory=2G --cpus=3 bitech/go-turbo-cachesrv


