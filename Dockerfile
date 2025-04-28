FROM golang:1.24

WORKDIR /app

COPY simple-app/ ./simple-app/
COPY lib/ ./lib/

WORKDIR /app/simple-app

RUN go mod tidy

RUN go build -o server ./cmd/main.go

CMD ["./server"]
