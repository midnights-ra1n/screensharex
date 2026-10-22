FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /screensharex .

FROM alpine:3.22
RUN apk add --no-cache ffmpeg ca-certificates tzdata \
 && adduser -D -H -u 1000 screensharex && mkdir /data && chown screensharex /data
COPY --from=server /screensharex /usr/bin/screensharex
USER screensharex
ENV SSX_DATA=/data SSX_ADDR=:8080
VOLUME /data
EXPOSE 8080
HEALTHCHECK CMD wget -qO /dev/null http://127.0.0.1:8080/api/public || exit 1
ENTRYPOINT ["/usr/bin/screensharex"]
