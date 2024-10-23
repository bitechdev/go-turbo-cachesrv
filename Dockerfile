FROM golang:1.23.2-alpine as go-turnbo-cachesrv
EXPOSE 8080

ENV LANG=en_ZA.UTF-8 \
    LANGUAGE=en_ZA.UTF-8 \
    LC_ALL=en_ZA.UTF-8

run apk update
# run apk add --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge
# run apk add yarn npm bash git openssh curl ca-certificates tzdata binutils-gold g++ gcc gnupg libgcc \
# libstdc++ linux-headers build-base nodejs make python3 icu-data-full libffi-dev openssl-dev pkgconfig glib-dev \
#  cairo-dev fribidi-dev zip

FROM go-turnbo-cachesrv as go-turnbo-cachesrv-stage

RUN mkdir /app
WORKDIR /app
COPY ./go.mod /app/go.mod
COPY ./main.go /app/main.go
ENV GOPRIVATE=github.com/bitechdev/*
ENV GONOSUMDB=*

RUN go mod download
RUN go mod tidy

RUN go build -o /app/server_bin

VOLUME [ "/data" ]

COPY ./docker_entrypoint.sh /app/bin/docker_entrypoint.sh
RUN chmod +x /app/bin/docker_entrypoint.sh


# RUN cd /tmp/restapi
# RUN go test

WORKDIR /app/bin
RUN cd /app/bin

ENTRYPOINT ["/bin/sh","./docker_entrypoint.sh"]
