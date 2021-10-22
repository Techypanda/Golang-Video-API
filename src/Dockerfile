FROM golang:1.17-alpine

COPY . /app/
WORKDIR /app/

RUN apk add curl
RUN curl -sSfL https://raw.githubusercontent.com/cosmtrek/air/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
RUN apk add  --no-cache ffmpeg

EXPOSE 1323