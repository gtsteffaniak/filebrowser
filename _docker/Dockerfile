FROM gtstef/ffmpeg:8.0.1-decode AS ffmpeg

FROM node:jod-slim AS nbuild
WORKDIR /app
COPY ./frontend/package.json ./
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates \
    && git config --global url."https://github.com/".insteadOf "git@github.com:" \
    && git config --global url."https://github.com/".insteadOf "ssh://git@github.com/"
RUN npm i --maxsockets 1
COPY ./frontend/ ./
RUN npm run build:docker

FROM golang:alpine AS base
ARG VERSION
ARG REVISION
WORKDIR /app
COPY ./backend ./
COPY --from=nbuild /app/dist/ ./http/embed/
RUN apk update && apk add --no-cache gcc musl-dev upx
ENV CGO_ENABLED=1
ENV GOEXPERIMENT=greenteagc
RUN go build -tags mupdf,musl -ldflags="-w -s \
  -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.Version=${VERSION}' \
  -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.CommitSHA=${REVISION}'" \
  -o filebrowser .
RUN upx filebrowser

FROM alpine:latest
COPY --from=ffmpeg [ "/ffmpeg", "/ffprobe", "/usr/local/bin/" ]
ENV FILEBROWSER_FFMPEG_PATH="/usr/local/bin/"
ENV FILEBROWSER_DATABASE="/home/filebrowser/data/database.db"
ENV PATH="$PATH:/home/filebrowser"
RUN apk --no-cache add ca-certificates mailcap tzdata curl exiftool
RUN adduser -D -s /bin/true -u 1000 filebrowser
USER filebrowser
WORKDIR /home/filebrowser
COPY --from=base --chown=filebrowser:1000 [ "/app/filebrowser", "./" ]
COPY --from=base --chown=filebrowser:1000 [ "/app/config.yaml", "./data/config.yaml" ]
COPY --from=base --chown=filebrowser:1000 [ "/app/config.yaml", "./" ]
COPY --from=base --chown=filebrowser:1000 [ "/app/reduce-rounded-corners.css", "./" ]
COPY --from=nbuild --chown=filebrowser:1000 [ "/app/dist/", "./http/dist/" ]
## sanity checks
RUN [ "filebrowser", "version" ]
RUN [ "ffmpeg", "-version" ]
RUN [ "ffprobe", "-version" ]
RUN [ "exiftool", "-ver" ]
USER root
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1
ENTRYPOINT [ "./filebrowser" ]
