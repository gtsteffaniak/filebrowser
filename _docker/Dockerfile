FROM gtstef/ffmpeg:8.1.2-decode AS ffmpeg
FROM golang:alpine AS base
ARG VERSION
ARG REVISION
WORKDIR /app
COPY ./backend ./
RUN apk update && apk add --no-cache gcc musl-dev
ENV CGO_ENABLED=1
RUN go build -tags analytics,mupdf,musl -ldflags="-w -s \
  -X 'github.com/gtsteffaniak/filebrowser/backend/internal/version.Version=${VERSION}' \
  -X 'github.com/gtsteffaniak/filebrowser/backend/internal/version.CommitSHA=${REVISION}'" \
  -o filebrowser .

FROM node:jod-slim AS nbuild
WORKDIR /build
COPY ./frontend/package.json ./frontend/
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates \
    && git config --global url."https://github.com/".insteadOf "git@github.com:" \
    && git config --global url."https://github.com/".insteadOf "ssh://git@github.com/"
RUN cd frontend && npm i --maxsockets 1
COPY ./frontend/ ./frontend/
WORKDIR /build/frontend
RUN npm run build:docker

FROM alpine:latest
COPY --from=ffmpeg [ "/ffmpeg", "/ffprobe", "/usr/local/bin/" ]
ENV FILEBROWSER_FFMPEG_PATH="/usr/local/bin/"
ENV FILEBROWSER_DATABASE_PATH="/home/filebrowser/data/database.sqlite"
ENV FILEBROWSER_CONFIG="/home/filebrowser/data/config.yaml"
ENV PATH="$PATH:/home/filebrowser"
RUN apk --no-cache add ca-certificates mailcap tzdata curl
RUN adduser -D -s /bin/true -u 1000 filebrowser
USER filebrowser
WORKDIR /home/filebrowser
# copy config to two locations for compatibility
COPY --from=base --chown=filebrowser:1000 [ "/app/filebrowser", "./" ]
COPY --from=base --chown=filebrowser:1000 [ "/app/config.yaml", "./data/config.yaml" ]
COPY --from=base --chown=filebrowser:1000 [ "/app/config.yaml", "./" ]
COPY --from=base --chown=filebrowser:1000 [ "/app/reduce-rounded-corners.css", "./" ]
COPY --from=nbuild --chown=filebrowser:1000 [ "/build/backend/internal/web/dist/", "./internal/web/dist/" ]
## sanity checks
RUN [ "filebrowser", "version" ]
RUN [ "ffmpeg", "-version" ]
RUN [ "ffprobe", "-version" ]
# exposing default port for auto discovery.
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:80/health || exit 1
ENTRYPOINT [ "./filebrowser" ]
