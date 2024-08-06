FROM golang:1.22-alpine as base
WORKDIR /app
COPY ./backend ./
RUN go build -ldflags="-w -s" -o filebrowser .

FROM node:slim as nbuild
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
