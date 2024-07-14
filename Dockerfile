FROM golang:alpine as build

WORKDIR /app

COPY . .

RUN go build -v -trimpath -o one-mirror ./

FROM alpine:latest as prod

WORKDIR /app

COPY --from=build /app/one-mirror /app/one-mirror
COPY --from=build /app/config.json /app/config.json
EXPOSE 8080

ENTRYPOINT [ "/app/one-mirror" ]
CMD        ["/app/config.json" ]