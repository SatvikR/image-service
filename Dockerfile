FROM golang:1.16.5 as build

WORKDIR /usr/src
COPY . .
ENV GO111MODULE=on
RUN CGO_ENABLED=0 GOOS=linux go build -o image-service

FROM alpine:latest

WORKDIR /app
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
COPY --from=build /usr/src/image-service .
ENV GIN_MODE=release
EXPOSE 8000
ENTRYPOINT [ "./image-service" ]
