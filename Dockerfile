FROM node:slim as nbuild
WORKDIR /app
COPY  ./frontend/package*.json ./
RUN npm i
COPY  ./frontend/ ./
RUN npm run build

FROM golang:1.21-alpine as base
WORKDIR /app
COPY  ./backend ./
RUN go build -ldflags="-w -s" -o filebrowser .

FROM alpine:latest
RUN apk --no-cache add \
      ca-certificates \
      mailcap
VOLUME /srv
EXPOSE 8080
WORKDIR /
COPY --from=base /app/settings/filebrowser.yaml /filebrowser.yaml
COPY --from=base /app/filebrowser /filebrowser
COPY --from=nbuild /app/dist/ /frontend/dist/
ENTRYPOINT [ "./filebrowser" ]