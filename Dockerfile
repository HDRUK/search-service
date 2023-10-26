FROM golang:latest

WORKDIR /app

COPY . .

RUN go mod download

RUN go build --ldflags='-s -w' -o search-service

EXPOSE 8080

CMD ["./search-service"]