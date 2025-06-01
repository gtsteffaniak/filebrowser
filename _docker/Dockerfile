FROM golang:1.24-alpine AS base
ARG VERSION
ARG REVISION
WORKDIR /app
COPY ./backend ./
#RUN swag init --output swagger/docs
RUN ln -s swagger /usr/local/go/src/
RUN go build -tags pdf -ldflags="-w -s \
  -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.Version=${VERSION}' \
  -X 'github.com/gtsteffaniak/filebrowser/backend/common/version.CommitSHA=${REVISION}'" \
  -o filebrowser .

FROM node:slim AS nbuild
WORKDIR /app
COPY ./frontend/package.json ./
RUN npm i --maxsockets 1
COPY  ./frontend/ ./
RUN npm run build-docker

FROM alpine:latest

RUN apk add --no-cache curl xz && \
    ARCH=$(uname -m) && \
    case "$ARCH" in \
      x86_64)   URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz" ;; \
      aarch64)  URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-arm64-static.tar.xz" ;; \
      armv7l)   URL="https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-armhf-static.tar.xz" ;; \
      *)        echo "Unsupported architecture: $ARCH" && exit 1 ;; \
    esac && \
    mkdir -p /tmp/ffmpeg && \
    curl -L "$URL" | tar -xJ -C /tmp/ffmpeg && \
    mv /tmp/ffmpeg/ffmpeg-*/ffmpeg /usr/local/bin/ && \
    mv /tmp/ffmpeg/ffmpeg-*/ffprobe /usr/local/bin/ && \
    chmod +x /usr/local/bin/ffmpeg /usr/local/bin/ffprobe && \
    rm -rf /tmp/ffmpeg && \
    apk del curl xz && \
    rm -rf /var/cache/apk/*

# requirements to enable pdf support
RUN apk add --no-cache libffi mupdf && \
    mv /usr/lib/libmupdf.so.25.6 /usr/lib/libmupdf.so

ENV FILEBROWSER_FFMPEG_PATH="/usr/local/bin/"
ENV FILEBROWSER_NO_EMBEDED="true"
ENV PATH="$PATH:/home/filebrowser"
RUN apk --no-cache add ca-certificates mailcap tzdata
RUN adduser -D -s /bin/true -u 1000 filebrowser
USER filebrowser
WORKDIR /home/filebrowser
COPY --from=base --chown=filebrowser:1000 /app/filebrowser* ./
COPY --from=base --chown=filebrowser:1000 /app/config.media.yaml ./config.yaml
COPY --from=nbuild --chown=filebrowser:1000 /app/dist/ ./http/dist/

## sanity checks
RUN [ "filebrowser", "version" ]
RUN [ "ffmpeg", "-version" ]
RUN [ "ffprobe", "-version" ]

USER root
# exposing default port for auto discovery.
EXPOSE 80
ENTRYPOINT [ "./filebrowser" ]
