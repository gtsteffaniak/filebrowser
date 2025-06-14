FROM gtstef/ffmpeg:7.1.1-decode AS ffmpeg
FROM golang:alpine AS base
ARG VERSION
ARG REVISION
WORKDIR /app
COPY ./backend ./
RUN apk update && apk add --no-cache gcc musl-dev upx
ENV CGO_ENABLED=1
RUN go build -tags mupdf,musl -ldflags="-w -s \
  -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.Version=${VERSION}' \
  -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.CommitSHA=${REVISION}'" \
  -o filebrowser .
RUN upx filebrowser

FROM node:lts-slim AS nbuild
WORKDIR /app
COPY ./frontend/package.json ./
RUN npm i --maxsockets 1
COPY  ./frontend/ ./
RUN npm run build-docker

FROM alpine:latest
COPY --from=ffmpeg [ "/ffmpeg", "/ffprobe", "/usr/local/bin/" ]
ENV FILEBROWSER_FFMPEG_PATH="/usr/local/bin/"
ENV FILEBROWSER_DATABASE="/home/filebrowser/data/database.db"
ENV FILEBROWSER_NO_EMBEDED="true"
ENV PATH="$PATH:/home/filebrowser"
RUN apk --no-cache add ca-certificates mailcap tzdata
RUN adduser -D -s /bin/true -u 1000 filebrowser
USER filebrowser
WORKDIR /home/filebrowser
COPY --from=base --chown=filebrowser:1000 /app/filebrowser ./
COPY --from=base --chown=filebrowser:1000 /app/config.yaml ./data/config.yaml
COPY --from=nbuild --chown=filebrowser:1000 /app/dist/ ./http/dist/
## sanity checks
RUN [ "filebrowser", "version" ]
RUN [ "ffmpeg", "-version" ]
RUN [ "ffprobe", "-version" ]
USER root
# exposing default port for auto discovery.
EXPOSE 80
ENTRYPOINT [ "./filebrowser" ]
