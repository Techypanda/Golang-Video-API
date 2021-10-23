FROM golang:1.17-alpine

COPY . /app/
WORKDIR /app/

RUN apk add curl
RUN apk add  --no-cache ffmpeg

EXPOSE 1323