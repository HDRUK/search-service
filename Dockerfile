FROM golang:latest

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build --ldflags='-s -w' -o search-service

EXPOSE 8080

CMD ["./search-service"]