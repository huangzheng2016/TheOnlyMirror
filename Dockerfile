FROM node:25-alpine as webbuild

WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend ./
RUN npm run build

FROM golang:1.22-alpine as build

WORKDIR /app

COPY . .
COPY --from=webbuild /app/frontend/dist /app/frontend/dist

RUN go build -v -trimpath -o the-only-mirror  ./

FROM alpine:latest as prod

WORKDIR /app

COPY --from=build /app/the-only-mirror  /app/the-only-mirror 
COPY --from=build /app/config.json /app/config.json
EXPOSE 8080

ENTRYPOINT [ "/app/the-only-mirror" ]