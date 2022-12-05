#syntax=docker/dockerfile:1

## Build
FROM golang:1.19.3-alpine AS build

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./

RUN go build -o lsweep-agent-proxy

## Runner
FROM alpine:latest

COPY --from=build /build/lsweep-agent-proxy /lsweep-agent-proxy

ENV DEBUG="false"
ENV LISTEN=""
ENV PUBLIC_PORT=""
ENV LISTEN_HOSTNAME="lsagent.example.com"
ENV LSSERVER_HOST="lansweeper.example.com"
ENV LSSERVER_PORT="9524"
ENV LSSERVER_CERT=""
ENV LSSERVER_IGNORE_CERT=""

EXPOSE 8011

CMD ["/lsweep-agent-proxy"]

HEALTHCHECK --timeout=3s --start-period=3s --retries=2 CMD wget http://localhost:8011/ctstatus -q -O - > /dev/null 2>&1