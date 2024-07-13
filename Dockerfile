FROM golang:1.20-alpine

WORKDIR /app

COPY . .

RUN go env -w GOPROXY=https://goproxy.cn,direct

RUN go mod download

RUN go build -o app

CMD ["./app"]