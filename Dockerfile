FROM golang:alpine as build

WORKDIR /app

COPY . .

RUN go build -v -trimpath -o the-only-mirror  ./

FROM alpine:latest as prod

WORKDIR /app

COPY --from=build /app/the-only-mirror  /app/the-only-mirror 
COPY --from=build /app/config.json /app/config.json
EXPOSE 8080

ENTRYPOINT [ "/app/the-only-mirror" ]