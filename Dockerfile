FROM node:slim as nbuild
WORKDIR /app
COPY  ./frontend/package*.json ./
RUN npm i
COPY  ./frontend/ ./
RUN npm run build

FROM golang:1.21-alpine as base
WORKDIR /app
COPY  ./backend ./
RUN go get -u golang.org/x/net
RUN go build -ldflags="-w -s" -o filebrowser .

FROM alpine:latest
ARG app="/app/filebrowser"
RUN apk --no-cache add \
      ca-certificates \
      mailcap
WORKDIR /app
COPY --from=base $app* ./
COPY --from=nbuild /app/dist/ ./frontend/dist/
ENTRYPOINT [ "./filebrowser" ]