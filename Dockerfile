FROM golang:1.22-alpine AS base
ARG VERSION
ARG REVISION
WORKDIR /app
COPY ./backend ./
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
COPY --from=base /app/filebrowser* ./
COPY --from=nbuild /app/dist/ ./frontend/dist/
ENTRYPOINT [ "./filebrowser" ]
