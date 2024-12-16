FROM golang:1.23-alpine AS base
ARG VERSION
ARG REVISION
WORKDIR /app
COPY ./backend ./
#RUN swag init --output swagger/docs
RUN ln -s swagger /usr/local/go/src/
RUN go build -ldflags="-w -s \
  -X 'github.com/gtsteffaniak/filebrowser/version.Version=${VERSION}' \
  -X 'github.com/gtsteffaniak/filebrowser/version.CommitSHA=${REVISION}'" \
  -o filebrowser .

FROM node:slim AS nbuild
WORKDIR /app
COPY ./frontend/package.json ./
RUN npm i --maxsockets 1
COPY  ./frontend/ ./
RUN npm run build-docker

FROM alpine:latest
ENV FILEBROWSER_NO_EMBEDED="true"
RUN apk --no-cache add ca-certificates mailcap
WORKDIR /home/filebrowser
RUN adduser -D -s /bin/true -u 1000 filebrowser
USER filebrowser
COPY --from=base --chown=filebrowser:1000 /app/filebrowser* ./
COPY --from=nbuild --chown=filebrowser:1000 /app/dist/ ./http/dist/
USER root
# exposing default port for auto discovery.
EXPOSE 80
ENTRYPOINT [ "./filebrowser" ]
