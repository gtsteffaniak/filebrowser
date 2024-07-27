FROM node:slim as nbuild
WORKDIR /app
COPY ./frontend/package*.json ./
RUN npm ci --maxsockets 1
COPY  ./frontend/ ./
RUN npm run build

FROM golang:1.22-alpine as base
WORKDIR /app
COPY ./backend ./
RUN go build -ldflags="-w -s" -o filebrowser .

FROM alpine:latest
ARG app="/app/filebrowser"
RUN apk --no-cache add ca-certificates mailcap
COPY --from=base /app/filebrowser* ./
COPY --from=nbuild /app/dist/ ./frontend/dist/
ENTRYPOINT [ "./filebrowser" ]
